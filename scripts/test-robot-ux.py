#!/usr/bin/env python3
"""Run only KeepSave's synthetic UX slice with cgroup and host observations.

The caller supplies the enforced systemd/container limits. This runner verifies
them; its watchdogs add bounded time/output/headroom guards, not new hard caps.
No credentials, request bodies, container environments or process arguments are
written to the resource receipt.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time
from urllib.parse import urlsplit
from urllib.request import build_opener, ProxyHandler, Request, HTTPRedirectHandler

GIB = 1024**3
MAX_OUTPUT = 200 * 1024**2
MAX_ACTIVE_SECONDS = 480
CGROUP_ROOT = Path("/sys/fs/cgroup")
ROOT = Path(__file__).resolve().parents[1]


def command(args: list[str]) -> str:
    result = subprocess.run(args, capture_output=True, text=True, timeout=5, check=False)
    if result.returncode:
        raise ValueError("Required local metadata command failed")
    return result.stdout.strip()


def cgroup_for_pid(pid: int) -> Path:
    for line in Path(f"/proc/{pid}/cgroup").read_text().splitlines():
        if line.startswith("0::"):
            group = (CGROUP_ROOT / line[3:].lstrip("/")).resolve()
            if not group.is_relative_to(CGROUP_ROOT) or not group.is_dir():
                break
            return group
    raise ValueError("A visible unified cgroup v2 group is required")


def controls(group: Path, soft_memory: bool = False) -> dict:
    values = {name: (group / name).read_text().strip() if (group / name).exists() else None
              for name in ("cpu.max", "memory.max", "memory.swap.max", "pids.max")}
    if not values["cpu.max"] or not values["pids.max"]:
        raise ValueError("Effective cgroup CPU and PID controls are mandatory in every mode")
    cpu = values["cpu.max"].split()
    if (len(cpu) != 2 or cpu[0] == "max" or int(cpu[0]) <= 0 or int(cpu[1]) <= 0
            or values["pids.max"] == "max" or int(values["pids.max"]) <= 0):
        raise ValueError("Every executor needs finite CPU/PID controls")
    if not soft_memory and (values["memory.max"] in (None, "max")
            or int(values["memory.max"]) <= 0 or values["memory.swap.max"] != "0"):
        raise ValueError("Every executor needs finite CPU/RAM/PID limits and zero swap")
    return {"path": str(group.relative_to(CGROUP_ROOT)), **values,
            "cpu_cores": int(cpu[0]) / int(cpu[1]),
            "RAM_enforcement": "soft RSS watchdog; no hard RAM assurance" if soft_memory else "hard cgroup limit"}


def process_rss(group: Path) -> int:
    """Conservative sum, including descendants; shared pages may be counted twice."""
    pids = set()
    for path in [group / "cgroup.procs", *group.glob("**/cgroup.procs")]:
        try:
            pids.update(int(value) for value in path.read_text().split())
        except FileNotFoundError:
            continue  # A descendant group may have exited since discovery.
    total = 0
    for pid in pids:
        try:
            for line in Path(f"/proc/{pid}/status").read_text().splitlines():
                if line.startswith("VmRSS:"):
                    total += int(line.split()[1]) * 1024
                    break
        except (FileNotFoundError, ProcessLookupError):
            continue
    return total


def counters(group: Path, soft_memory: bool = False) -> dict:
    cpu = dict(line.split() for line in (group / "cpu.stat").read_text().splitlines())
    event_file = group / "memory.events"
    events = dict(line.split() for line in event_file.read_text().splitlines()) if event_file.exists() else {}
    rss = process_rss(group) if soft_memory else None
    memory = rss if soft_memory else int((group / "memory.current").read_text())
    return {"memory_bytes": memory,
            "memory_measurement": "sum process VmRSS, shared pages double-counted" if soft_memory else "cgroup memory.current",
            "process_RSS_bytes": rss,
            "pids": int((group / "pids.current").read_text()),
            "cpu_usage_usec": int(cpu["usage_usec"]),
            "cpu_throttled_usec": int(cpu.get("throttled_usec", 0)),
            "OOM_counters_available": event_file.exists(),
            "oom_events": int(events.get("oom", 0)),
            "oom_kill_events": int(events.get("oom_kill", 0))}


def pi_health() -> dict:
    temperature = int(Path("/sys/class/thermal/thermal_zone0/temp").read_text()) / 1000
    raw = command(["vcgencmd", "get_throttled"])
    if not re.fullmatch(r"throttled=0x[0-9a-fA-F]+", raw):
        raise ValueError("Pi firmware health could not be verified")
    return {"temperature_C": temperature, "throttled_flags": int(raw.split("=", 1)[1], 16)}


def source_identity() -> dict:
    manifest = ROOT / "source-manifest.json"
    if not manifest.exists():
        return {"git_revision": command(["git", "-C", str(ROOT), "rev-parse", "HEAD"]),
                "provenance": "checkout base revision; working-tree edits may be present"}
    if manifest.stat().st_size > 1024 * 1024:
        raise ValueError("Source manifest exceeds one MiB")
    raw = manifest.read_bytes()
    data = json.loads(raw)
    revision = data.get("base_revision")
    hashes = data.get("sha256")
    if not isinstance(revision, str) or not re.fullmatch(r"[0-9a-fA-F]{7,64}", revision) or not isinstance(hashes, dict):
        raise ValueError("Source manifest requires a base_revision and sha256 map")
    required = {"scripts/test-robot-ux.py", "tests/robot/libraries/KeepSaveUX.py",
                "tests/robot/tests/04_application_ux.robot", "tests/robot/tests/05_application_behaviour.robot"}
    if not required.issubset(hashes):
        raise ValueError("Source manifest must identify the runner, fixture library and both suites")
    for name, digest in hashes.items():
        path = (ROOT / name).resolve()
        if (not path.is_relative_to(ROOT) or not path.is_file() or not isinstance(digest, str)
                or not re.fullmatch(r"[0-9a-fA-F]{64}", digest)
                or hashlib.sha256(path.read_bytes()).hexdigest() != digest.lower()):
            raise ValueError("Transferred source does not match its bounded source manifest")
    return {"base_revision": revision, "manifest_sha256": hashlib.sha256(raw).hexdigest(),
            "verified_files": len(hashes), "provenance": "hash-verified transferred files; base revision is source provenance, not a remote checkout assertion"}


def host_counters() -> dict:
    memory = dict(line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())
    cpu = [int(value) for value in Path("/proc/stat").read_text().splitlines()[0].split()[1:9]]
    return {"available_bytes": int(memory["MemAvailable"].split()[0]) * 1024,
            "cpu_total_ticks": sum(cpu), "cpu_idle_ticks": cpu[3]}


def output_size(path: Path) -> int:
    total = 0
    for folder, _, files in os.walk(path, followlinks=False):
        for name in files:
            candidate = Path(folder) / name
            try:
                info = candidate.lstat()
            except FileNotFoundError:
                continue  # Browser output may be atomically replaced or removed.
            if stat.S_ISREG(info.st_mode):
                total += info.st_size
    return total


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def readiness(origin: str) -> None:
    opener = build_opener(ProxyHandler({}), NoRedirect())
    for path in ("/", "/api/v1/auth/providers"):
        with opener.open(Request(origin + path, method="GET"), timeout=10) as response:
            if response.status != 200 or len(response.read(1024 * 1024 + 1)) > 1024 * 1024:
                raise ValueError("Application readiness probe failed")


def stop_child(child: subprocess.Popen) -> bool:
    """Signal only this runner's newly created process group, never services."""
    try:
        os.killpg(child.pid, signal.SIGTERM)
    except ProcessLookupError:
        return True
    try:
        child.wait(timeout=5)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        child.wait(timeout=5)
    # A dead parent alone does not prove Chromium children exited.
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        try:
            os.killpg(child.pid, 0)
        except ProcessLookupError:
            return True
        time.sleep(0.1)
    return False


def run(args: argparse.Namespace, receipt: dict) -> int:
    fixture_path = Path(args.fixture).resolve(strict=True)
    fixture_info = fixture_path.stat()
    if fixture_info.st_uid != os.getuid() or fixture_info.st_mode & 0o077:
        raise ValueError("Synthetic fixture must be owned by the caller and mode 0600 or stricter")
    fixture = json.loads(fixture_path.read_text())
    origin = urlsplit(args.origin)
    if (origin.scheme != "http" or origin.hostname != "127.0.0.1" or not origin.port
            or origin.path not in ("", "/") or origin.query or origin.fragment
            or origin.username or origin.password or args.origin != fixture.get("origin")
            or fixture.get("sample_data") is not True):
        raise ValueError("Exact explicitly synthetic loopback origin is required")
    python = Path(args.python).absolute()
    browser = Path(args.browser).absolute()
    if not all(path.is_file() and os.access(path, os.X_OK) for path in (python, browser)):
        raise ValueError("Supply existing executable venv Python and matching Chromium paths")
    if len(args.containers) != 2 or len(set(args.containers)) != 2:
        raise ValueError("Supply the two distinct explicitly owned API and PostgreSQL containers")
    if not all(re.fullmatch(r"ks-gargantua-[A-Za-z0-9-]+-(?:api|db)", name)
               for name in args.containers):
        raise ValueError("Container names must belong to the dedicated Gargantua demo")

    if args.soft_memory_fallback and args.supervisor_pid is None:
        raise ValueError("Pi soft-memory mode requires --supervisor-pid for whole-stack RSS coverage")
    groups = {"runner": cgroup_for_pid(os.getpid())}
    for name in args.containers:
        pid = int(command(["docker", "inspect", "--format", "{{.State.Pid}}", name]))
        if pid <= 0:
            raise ValueError("A named test container is not running")
        groups[name] = cgroup_for_pid(pid)
    if args.supervisor_pid is not None:
        if args.supervisor_pid <= 0 or Path(f"/proc/{args.supervisor_pid}").stat().st_uid != os.getuid():
            raise ValueError("The supplied proxy supervisor must be an existing caller-owned process")
        groups["proxy"] = cgroup_for_pid(args.supervisor_pid)
    group_paths = list(groups.values())
    for index, group in enumerate(group_paths):
        if any(group.is_relative_to(other) or other.is_relative_to(group)
               for other in group_paths[index + 1:]):
            raise ValueError("Observed executor cgroups must be distinct and non-overlapping")
    receipt["controls"] = {name: controls(group, args.soft_memory_fallback) for name, group in groups.items()}
    runner_limits = receipt["controls"]["runner"]
    runner_cpu, runner_pids = (0.650001, 176) if args.soft_memory_fallback else (1, 256)
    service_cpu, service_pids = (0.300001, 56) if args.soft_memory_fallback else (0.500001, 128)
    if (runner_limits["cpu_cores"] > runner_cpu
            or int(runner_limits["pids.max"]) > runner_pids
            or (not args.soft_memory_fallback and int(runner_limits["memory.max"]) > 2 * GIB)):
        raise ValueError("Runner CPU/PID or hard-memory controls exceed the selected test envelope")
    service_limits = [receipt["controls"][name] for name in args.containers]
    if (sum(value["cpu_cores"] for value in service_limits) > service_cpu
            or sum(int(value["pids.max"]) for value in service_limits) > service_pids
            or (not args.soft_memory_fallback and sum(int(value["memory.max"]) for value in service_limits) > GIB)):
        raise ValueError("API/PostgreSQL CPU/PID or hard-memory controls exceed the selected test envelope")
    if "proxy" in groups:
        proxy = receipt["controls"]["proxy"]
        proxy_cpu, proxy_pids = (0.050001, 24) if args.soft_memory_fallback else (0.100001, 64)
        if (proxy["cpu_cores"] > proxy_cpu or int(proxy["pids.max"]) > proxy_pids
                or (not args.soft_memory_fallback and int(proxy["memory.max"]) > 128 * 1024**2)):
            raise ValueError("Proxy supervisor controls exceed the selected test envelope")
    # A finite cgroup is not a finite elapsed deadline. Verify the owning unit.
    units = [part for part in groups["runner"].parts if part.endswith(".service")]
    if not units:
        raise ValueError("Run inside a finite-runtime user systemd service")
    runtime = command(["systemctl", "--user", "show", units[-1],
                       "--property=RuntimeMaxUSec", "--value"])
    if runtime in ("", "infinity", "0"):
        raise ValueError("The owning systemd service needs a finite RuntimeMaxSec")
    receipt["systemd_runtime_limit"] = runtime
    receipt["origin"] = args.origin
    receipt["source_identity"] = source_identity()
    receipt["scope"] = {
        "suites": ["04_application_ux.robot", "05_application_behaviour.robot"],
        "workers": 1, "browser": "one owned headless Chromium, sequential suites",
        "services": "existing synthetic API/PostgreSQL/proxy; never stopped by this runner",
        "proxy_measurement": "included via caller-owned supervisor cgroup" if "proxy" in groups else "not supplied: proxy is outside usage samples and aggregate controls",
        "shared_browser": "The user-visible IAB is outside this cgroup and is not hard capped",
        "disk_guard": "200 MiB soft output watchdog; can overshoot between samples",
        "capacity_claim": False,
        "memory_profile": "owner-authorized Pi soft RSS fallback; hard RAM cap absent" if args.soft_memory_fallback else "cgroup hard RAM profile",
    }
    receipt["aggregate_configured"] = {
        "coverage": "runner/API/PostgreSQL/proxy; shared IAB excluded" if "proxy" in groups else "runner/API/PostgreSQL only; proxy and shared IAB excluded",
        "cpu_cores": sum(item["cpu_cores"] for item in receipt["controls"].values()),
        "memory_bytes": None if args.soft_memory_fallback else sum(int(item["memory.max"]) for item in receipt["controls"].values()),
        "pids": sum(int(item["pids.max"]) for item in receipt["controls"].values()),
    }
    readiness(args.origin)
    output = Path(args.output).resolve()
    start = time.monotonic()
    samples = receipt["samples"] = []
    previous_host = None
    initial_firmware_flags = None
    low_idle_since = None
    child = None
    interrupted = None

    def interrupted_signal(signum, _frame):
        nonlocal interrupted
        interrupted = signum

    for signum in (signal.SIGTERM, signal.SIGINT):
        signal.signal(signum, interrupted_signal)

    def sample(phase: str) -> str | None:
        nonlocal previous_host, low_idle_since, initial_firmware_flags
        now = time.monotonic()
        host = host_counters()
        idle = None
        if previous_host:
            delta = host["cpu_total_ticks"] - previous_host["cpu_total_ticks"]
            if delta > 0:
                idle = 100 * (host["cpu_idle_ticks"] - previous_host["cpu_idle_ticks"]) / delta
        previous_host = host
        if idle is not None and idle < 10:
            low_idle_since = low_idle_since or now
        else:
            low_idle_since = None
        row = {"seconds": round(now - start, 3), "phase": phase,
               "host_available_bytes": host["available_bytes"], "host_cpu_idle_percent": idle,
               "output_bytes": output_size(output),
               "groups": {name: counters(group, args.soft_memory_fallback) for name, group in groups.items()}}
        if args.soft_memory_fallback:
            row["Pi_health"] = pi_health()
            row["aggregate_conservative_RSS_bytes"] = sum(item["process_RSS_bytes"] for item in row["groups"].values())
            flags = row["Pi_health"]["throttled_flags"]
            if initial_firmware_flags is None:
                initial_firmware_flags = flags
        samples.append(row)
        if interrupted:
            return "runner_signal"
        if host["available_bytes"] < 8 * GIB:
            return "host_reserve_below_8_GiB"
        if low_idle_since is not None and now - low_idle_since >= 30:
            return "host_CPU_idle_below_10_percent_for_30_seconds"
        if row["output_bytes"] > MAX_OUTPUT:
            return "soft_output_budget_exceeded"
        if args.soft_memory_fallback:
            if row["aggregate_conservative_RSS_bytes"] > 2 * GIB:
                return "soft_aggregate_RSS_above_2_GiB"
            if row["Pi_health"]["temperature_C"] >= 70:
                return "Pi_temperature_at_least_70_C"
            if flags & 0xF:
                return "Pi_current_firmware_fault"
            if (flags & 0xF0000) & ~(initial_firmware_flags & 0xF0000):
                return "Pi_newly_latched_firmware_fault"
        for name, counts in row["groups"].items():
            baseline = samples[0]["groups"][name]
            if (counts["oom_events"] > baseline["oom_events"]
                    or counts["oom_kill_events"] > baseline["oom_kill_events"]):
                return "executor_OOM_event"
        return None

    try:
        baseline_deadline = start + 5
        while True:
            event = sample("baseline")
            if event:
                receipt["stop_reason"] = event
                return 125
            if time.monotonic() >= baseline_deadline:
                break
            time.sleep(min(2, baseline_deadline - time.monotonic()))
        env = os.environ.copy()
        env.update(KEEPSAVE_UI_URL=args.origin, KEEPSAVE_UX_FIXTURE=str(fixture_path),
                   KEEPSAVE_CHROMIUM=str(browser))
        log = output / "robot-console.log"
        active_start = time.monotonic()
        with log.open("wb") as stream:
            child = subprocess.Popen([str(python), "-m", "robot", "--outputdir", str(output),
                "--console", "quiet", "--loglevel", "INFO", "--name", "KeepSave Synthetic UX",
                str(ROOT / "tests/robot/tests/04_application_ux.robot"),
                str(ROOT / "tests/robot/tests/05_application_behaviour.robot")],
                cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT, start_new_session=True)
            receipt["robot_started"] = True
            while child.poll() is None:
                event = sample("active")
                if time.monotonic() - active_start >= MAX_ACTIVE_SECONDS:
                    event = "480_second_active_deadline"
                if event:
                    receipt["stop_reason"] = event
                    receipt["owned_process_group_stopped"] = stop_child(child)
                    receipt["robot_exit_code"] = child.returncode
                    return child.returncode if child.returncode > 0 else 124
                time.sleep(2)
            completion_event = sample("completed")
            receipt["robot_exit_code"] = child.returncode
            receipt["active_seconds"] = round(time.monotonic() - active_start, 3)
            receipt["owned_process_group_stopped"] = stop_child(child)
            if not receipt["owned_process_group_stopped"]:
                receipt["stop_reason"] = "owned_process_group_cleanup_unconfirmed"
                return 125
            if completion_event:
                receipt["stop_reason"] = completion_event
                return child.returncode or 125
            return child.returncode
    finally:
        if child is not None and child.poll() is None:
            receipt["owned_process_group_stopped"] = stop_child(child)
        receipt["measured_seconds"] = round(time.monotonic() - start, 3)
        if samples:
            receipt["peaks"] = {
                name: {"memory_bytes": max(row["groups"][name]["memory_bytes"] for row in samples),
                       "pids": max(row["groups"][name]["pids"] for row in samples),
                       "CPU_seconds_delta": (samples[-1]["groups"][name]["cpu_usage_usec"]
                                             - samples[0]["groups"][name]["cpu_usage_usec"]) / 1e6}
                for name in groups}
            receipt["minimum_host_available_bytes"] = min(row["host_available_bytes"] for row in samples)
            if args.soft_memory_fallback:
                receipt["peak_aggregate_conservative_RSS_bytes"] = max(row["aggregate_conservative_RSS_bytes"] for row in samples)
                receipt["peak_Pi_temperature_C"] = max(row["Pi_health"]["temperature_C"] for row in samples)
            receipt["output_bytes_before_receipt"] = output_size(output)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("python", "fixture", "browser", "origin", "output"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--containers", nargs=2, required=True, metavar=("API", "POSTGRES"))
    parser.add_argument("--supervisor-pid", type=int,
                        help="Existing caller-owned proxy supervisor PID; required for Pi soft-memory whole-stack coverage")
    parser.add_argument("--soft-memory-fallback", action="store_true",
                        help="Explicit owner-authorized Pi-only CPU/PID plus soft RSS/thermal watchdog; never a hard RAM guarantee")
    args = parser.parse_args()
    os.umask(0o077)
    output = Path(args.output).absolute()
    output.mkdir(mode=0o700, parents=True, exist_ok=True)
    if output.is_symlink() or output.stat().st_uid != os.getuid() or output.stat().st_mode & 0o077:
        parser.error("Output must be a private caller-owned directory")
    if any(output.iterdir()):
        parser.error("Output must be empty; preserve prior failed receipts in their own directory")
    receipt = {"format_version": 1, "started_unix": time.time(), "robot_started": False,
               "limitation": "Sampled single-user synthetic observation, not a load/capacity benchmark or causal learning comparison"}
    try:
        result = run(args, receipt)
    except Exception as error:
        # Do not serialize arbitrary exceptions, subprocess stderr or fixture data.
        receipt["failure"] = {"type": type(error).__name__,
                              "message": str(error) if type(error) is ValueError else
                              "Preflight or execution failed; no credentials were recorded"}
        result = 125
    receipt["exit_code"] = result
    receipt["finished_unix"] = time.time()
    (output / "resource-results.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps({"exit_code": result, "robot_started": receipt["robot_started"],
                      "report": str(output / "report.html"),
                      "resources": str(output / "resource-results.json")}))
    return result


if __name__ == "__main__":
    sys.exit(main())
