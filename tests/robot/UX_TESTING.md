# Application UX and behaviour acceptance

The [25 October acceptance record](../../docs/validation/2026-10-25-documentation-ux/README.md)
contains the executed synthetic scenarios, measured resource scope, retained
failures and limitations. This guide describes reproduction, not an automatic pass.

The `04_application_ux.robot` and `05_application_behaviour.robot` suites test the
actual frontend and API on a synthetic local instance. They cover landing-story
controls, keyboard navigation, responsive documentation, theme changes during an
unsaved edit, persistence/history, authentication denial, environment separation,
session revocation and twelve serial reads. They do not qualify real provider
login, SMTP, a production database, universal accessibility or system capacity.

The suites create their own disposable projects and tombstone them in teardown.
The existing sample workspace and the user's browser session remain separate.
An interrupted process may leave a `Robot UX …` project for explicitly scoped
cleanup; it must not trigger broad project/database deletion.

## Dependencies and destination

Install the pinned [requirements](requirements.txt) into a dedicated virtual
environment. Robot Framework is `7.4.2`; Browser is `20.0.0`. Use the Node version
supported by that Browser release and initialize its Node dependencies without
downloading another browser:

```sh
python3 -m venv /private/test-run/venv
/private/test-run/venv/bin/python -m pip install -r tests/robot/requirements.txt
/private/test-run/venv/bin/rfbrowser init --skip-browsers
```

Dependency installation is a separate bounded phase. Supply an existing
Chromium executable matching Browser's installed Playwright revision; a random
system Chrome version is not qualification evidence. The prepared workstation
cache contains Chromium revision `1223`; a prepared cache is not an executed
acceptance result. Record the actual selected executable/version with each run.
Do not change or upgrade the user's ordinary browser profile.

The fixture is a caller-owned mode-`0600` JSON file, in a private directory. It
contains `origin`, `sample_data: true`, and synthetic `email`/`password` fields.
Both the fixture and runner reject remote destinations before application
requests. Its origin must exactly match the supplied `http://127.0.0.1:PORT`
origin. Never use production credentials or commit the fixture. Robot keywords
read credentials internally and suppress login keyword arguments.

## Bounded invocation

Run from the repository root after the synthetic frontend/API/PostgreSQL stack
is ready. Replace every example path and both container names with the exact
owned test resources. The wrapper rejects unbounded executors before Robot
starts; it does not repair a missing cgroup controller or alter service limits.

```sh
systemd-run --user --wait --pipe --collect \
  --unit=keepsave-robot-ux-UNIQUE \
  --property=CPUQuota=100% \
  --property=MemoryMax=2G --property=MemorySwapMax=0 \
  --property=TasksMax=256 --property=RuntimeMaxSec=600 \
  /usr/bin/python3 scripts/test-robot-ux.py \
    --python /private/test-run/venv/bin/python \
    --fixture /private/demo/demo-context.json \
    --browser /private/cache/chromium-1223/chrome-linux64/chrome \
    --origin http://127.0.0.1:4663 \
    --output /private/test-run/robot-report-UNIQUE \
    --supervisor-pid EXACT_PROXY_SUPERVISOR_PID \
    --containers ks-gargantua-try-EXACT-api ks-gargantua-try-EXACT-db
```

Ensure the inherited `PATH` includes the matching Node executable. The output
directory must be private and empty; retries use a new directory so prior
failures are retained. The runner preserves the Robot exit status on normal
completion and failing tests. Preflight/monitor failures return `125`; a killed
deadline/guarded run returns a nonzero status and retains the Robot exit signal.

The local profile allocates one CPU, 2 GiB RAM, no swap and 256 PIDs to the test
process tree. Existing API/PostgreSQL/proxy allocations must be counted too.
For the 25 October slice those services totalled 0.6 CPU, 1,152 MiB and 192 PIDs:
the aggregate plan is 1.6 CPU, 3.125 GiB and 448 PIDs. The user-visible shared IAB
is outside these hard caps. Only one automated Chromium worker runs; suites are
sequential. These numbers are protective test budgets, not application sizing.

The wrapper verifies effective cgroup-v2 CPU, memory, zero-swap and PID controls
for its own group, both named containers and the supplied proxy-supervisor group.
It selects only each container's PID from Docker inspection. No container
environment is read. All observed groups must be distinct and non-overlapping.
The optional `--supervisor-pid` must identify the existing caller-owned proxy
process. Omitting it in strict mode explicitly records the proxy measurement
gap; its separate allocation still counts toward the whole-stack plan. It verifies a
finite systemd runtime, then probes the same-origin `/` and
`/api/v1/auth/providers` endpoints with ten-second deadlines and bounded reads.

Five baseline seconds precede Robot. Two-second samples record executor CPU,
memory, PIDs, OOM counters, output growth and host available memory/CPU idle.
The watchdog stops only the newly owned Robot process group on these events:

- Host available memory falls below 8 GiB.
- CPU idle stays below 10% for thirty seconds.
- Active Robot time reaches 480 seconds, or the runner receives termination.
- Output exceeds 200 MiB, an OOM event appears, or monitoring fails.

The output-size check is a soft guard and can overshoot between samples. The
systemd/runtime controls are the hard executor boundary. A watchdog stop does
not stop existing services or erase synthetic data. Chromium descendants are
closed with the owned process group; no global process-name kill is used.

## Explicit Raspberry Pi soft-memory profile

This fallback is **off by default** and requires explicit authorization for the
installation and run. It is for a Pi whose memory controller is unavailable;
the normal workstation profile continues to require effective hard RAM limits
and no swap. Passing this fallback cannot qualify a hard-RAM isolation claim.

After the Pi read-only health/capacity check, the 25 October 2026 authorized
profile reserves one CPU and 256 PIDs for the complete test stack:

| Executor | CPU quota | PID cap |
| --- | ---: | ---: |
| Robot, Node and its Chromium descendants | 0.65 CPU | 176 |
| KeepSave API | 0.15 CPU | 24 |
| Disposable PostgreSQL | 0.15 CPU | 32 |
| Proxy and independent watchdog | 0.05 CPU | 24 |

Use `CPUQuota=65%`, `TasksMax=176`, a finite `RuntimeMaxSec=600`, and explicitly
add `--soft-memory-fallback --supervisor-pid EXACT_PROXY_SUPERVISOR_PID` to the
runner command. The supervisor PID is mandatory in this profile. On the memory-controller
unsupported host, omit ineffective `MemoryMax`/`MemorySwapMax` declarations;
their absence is recorded. The existing containers must have actual finite CPU
and PID limits. The supervisor must have an effective quota at most 0.05 CPU and
24 PIDs. A separately bounded supervisor/watchdog must include dependencies,
startup and teardown in its whole-phase monitoring and cancellation. During
Robot execution the runner observes all four groups: Robot, API, PostgreSQL
and proxy/watchdog. Their combined ceiling is one CPU and 256 PIDs.

For each group, the fallback recursively discovers `cgroup.procs`, reads
`/proc/PID/status` and sums `VmRSS`. Shared pages can be counted more than once:
this is a conservative RSS observation, not physical usage or PSS. At two-second
intervals it stops its owned Robot group if all four groups' combined observed RSS exceeds
2 GiB, host available memory falls below 8 GiB, Pi temperature reaches 70°C,
firmware current-fault bits become nonzero, or a new firmware-history fault bit
appears. Five baseline seconds precede the suite. CPU, elapsed/output and
monitor-failure guards still apply. RSS/host/thermal guards can overshoot between
samples; **there is no hard memory cap and no hard zero-swap guarantee**. Missing
OOM counters are recorded as unavailable, not as proof that no OOM occurred.

A transferred source tree need not contain `.git`. Put a `source-manifest.json`
in its root with the originating `base_revision` and a `sha256` map from exact
relative file paths to SHA-256 hex digests. Include at least the runner, fixture
library and both selected suites; include every other transferred source/build
artifact relevant to the run. The runner validates supplied hashes and records
the manifest digest and base provenance. It does not assert that the remote tree
is a Git checkout or that uncommitted source changes belong to the base commit.

```json
{
  "base_revision": "ORIGINATING_GIT_REVISION",
  "sha256": {
    "scripts/test-robot-ux.py": "SHA256_HEX",
    "tests/robot/libraries/KeepSaveUX.py": "SHA256_HEX",
    "tests/robot/tests/04_application_ux.robot": "SHA256_HEX",
    "tests/robot/tests/05_application_behaviour.robot": "SHA256_HEX"
  }
}
```

The Pi's ordinary Chromium cache revision `1234` does not substitute for
Browser 20's matching revision. Install the matching ARM64 build in the bounded
dependency phase, preserving the user's existing Node/browser tools and services.

## Inspecting evidence

Open the generated `report.html` for the suite overview, `log.html` for keyword
details/screenshots, and `output.xml` for machine-readable results. These are
local artifacts inside the chosen output directory; do not publish them without
review. `robot-console.log` retains bounded console output. `resource-results.json`
records verified controls, source revision, baseline/active samples, sampled
peaks, CPU-time deltas, stop events and process-group cleanup. `api-timings-*.json`
contains only method/status/time/size metadata for synthetic operations.
Its configured aggregate covers all supplied executor groups. The Pi profile
requires all four groups. In strict mode, if the optional supervisor PID is
omitted, the receipt explicitly excludes the proxy from usage/aggregate samples;
its 0.1 CPU/128 MiB/64-PID allocation still belongs in the whole-stack plan.

Compare samples against the stated envelope and report actual pass/fail/skip
results. Sampled peaks can miss shorter spikes. The server metrics include the
existing sample instance's baseline activity; they cannot isolate each UI
interaction's cost or establish a throughput, availability, causal improvement
or production-capacity claim. Failed test runs remain failed even when resource
use stays inside its limits. Real Google/GitHub consent and production recovery
acceptance remain separate exercises.
