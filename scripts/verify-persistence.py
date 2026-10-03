#!/usr/bin/env python3
"""Exercise the real binary, embedded migrations, and restart persistence.

Uses disposable local SQLite or a disposable postgres:16 Docker container.
Generates all credentials and test values in memory; prints assertions only.
Run: python3 scripts/verify-persistence.py /path/to/keepsave --database sqlite
"""

import argparse
import base64
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import socket
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--database", choices=["sqlite", "postgres"], default="sqlite")
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    container = None
    process = None
    results = {"database": args.database, "checks": []}

    with tempfile.TemporaryDirectory(prefix="keepsave-persistence-") as temp:
        temp = Path(temp)
        port = free_port()
        base = f"http://127.0.0.1:{port}"
        database_url = f"sqlite://{temp / 'vault.db'}"
        try:
            if args.database == "postgres":
                container = "keepsave-persistence-" + secrets.token_hex(5)
                pg_password = secrets.token_urlsafe(32)
                docker_env = {**os.environ, "POSTGRES_PASSWORD": pg_password}
                subprocess.run([
                    "docker", "run", "--rm", "-d", "--name", container,
                    "--cpus=1", "--memory=256m", "-p", "127.0.0.1::5432",
                    "-e", "POSTGRES_PASSWORD", "-e", "POSTGRES_DB=keepsave",
                    "postgres:16",
                ], check=True, env=docker_env, stdout=subprocess.DEVNULL)
                address = subprocess.check_output(["docker", "port", container, "5432"], text=True).strip()
                database_url = f"postgres://postgres:{pg_password}@{address}/keepsave?sslmode=disable"
                for _ in range(100):
                    ready = subprocess.run(["docker", "exec", container, "pg_isready", "-h", "127.0.0.1", "-U", "postgres", "-d", "keepsave"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                    if ready.returncode == 0:
                        break
                    time.sleep(0.1)
                else:
                    raise RuntimeError("disposable PostgreSQL did not become ready")

            env = {
                "PATH": os.environ["PATH"], "DATABASE_URL": database_url,
                "MASTER_KEY": base64.b64encode(secrets.token_bytes(32)).decode(),
                "JWT_SECRET": secrets.token_urlsafe(48), "PORT": str(port),
                "GIN_MODE": "release", "GOMAXPROCS": "2", "GOMEMLIMIT": "128MiB",
                "CORS_ORIGINS": base, "DB_MAX_OPEN_CONNS": "4", "DB_MAX_IDLE_CONNS": "1",
            }

            def request(method, path, body=None, token=None, expected=200):
                headers = {"Content-Type": "application/json"}
                if token:
                    headers["Authorization"] = "Bearer " + token
                data = json.dumps(body).encode() if body is not None else None
                req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
                try:
                    response = urllib.request.urlopen(req, timeout=10)
                except urllib.error.HTTPError as error:
                    response = error
                with response:
                    if response.code != expected:
                        raise AssertionError(f"{method} {path}: expected {expected}, got {response.code}")
                    return json.load(response) if response.code != 204 else None

            def start():
                with open(temp / "server.log", "ab") as log:
                    proc = subprocess.Popen([str(binary)], cwd=temp, env=env, stdout=log, stderr=log)
                try:
                    for _ in range(100):
                        if proc.poll() is not None:
                            raise RuntimeError("API exited before readiness")
                        try:
                            request("GET", "/readyz")
                            return proc
                        except (urllib.error.URLError, TimeoutError):
                            time.sleep(0.1)
                    raise RuntimeError("API readiness timed out")
                except BaseException:
                    if proc.poll() is None:
                        proc.terminate()
                        proc.wait(timeout=15)
                    raise

            def memory_cpu():
                stat = Path(f"/proc/{process.pid}/stat").read_text().split()
                cpu = (int(stat[13]) + int(stat[14])) / os.sysconf("SC_CLK_TCK")
                status = Path(f"/proc/{process.pid}/status").read_text().splitlines()
                rss = next(int(line.split()[1]) for line in status if line.startswith("VmRSS:"))
                return cpu, rss / 1024

            process = start()
            email = secrets.token_hex(8) + "@example.invalid"
            password = secrets.token_urlsafe(24) + "aA1!"
            token = request("POST", "/api/v1/auth/register", {"email": email, "password": password}, expected=201)["token"]
            project = request("POST", "/api/v1/projects", {"name": "Persistence check", "description": "Disposable local verification"}, token, 201)["project"]
            project_id = project["id"]
            path = f"/api/v1/projects/{project_id}"
            first, updated = secrets.token_urlsafe(40), secrets.token_urlsafe(40)
            secret = request("POST", path + "/secrets", {"key": "PERSISTENCE_PROBE", "value": first, "environment": "alpha"}, token, 201)["secret"]
            secret_path = path + "/secrets/" + secret["id"]
            request("PUT", secret_path, {"value": updated}, token)
            values = request("GET", path + "/secrets?environment=alpha", token=token)["secrets"]
            assert len(values) == 1 and values[0]["value"] == updated, "updated value did not round-trip"
            assert request("GET", path + "/secrets?environment=prod", token=token)["secrets"] == [], "environment isolation failed"
            request("GET", path + "/secrets?environment=alpha", expected=401)
            outsider = request("POST", "/api/v1/auth/register", {"email": secrets.token_hex(8) + "@example.invalid", "password": password}, expected=201)["token"]
            request("GET", path + "/secrets?environment=alpha", token=outsider, expected=403)
            results["checks"].extend(["register/create/update/read", "environment isolation", "unauthenticated and cross-tenant denial"])

            def import_value(value, overwrite=False):
                return request("POST", path + "/env-import", {
                    "environment": "uat", "content": "IMPORT_PROBE=" + value,
                    "overwrite": overwrite,
                }, token)["result"]

            assert import_value(first)["created"] == ["IMPORT_PROBE"]
            assert import_value(updated)["skipped"] == ["IMPORT_PROBE"]
            assert request("GET", path + "/secrets?environment=uat", token=token)["secrets"][0]["value"] == first
            assert import_value(updated, overwrite=True)["updated"] == ["IMPORT_PROBE"]
            results["checks"].append("env import creates, skips, and overwrites correctly")

            process.terminate()
            process.wait(timeout=15)
            process = start()
            # Existing token and new login both work across process restart;
            # this also exercises persistence of the signing and envelope keys.
            assert request("GET", secret_path, token=token)["secret"]["value"] == updated
            token = request("POST", "/api/v1/auth/login", {"email": email, "password": password})["token"]
            assert request("GET", secret_path, token=token)["secret"]["value"] == updated
            assert request("GET", path + "/secrets?environment=uat", token=token)["secrets"][0]["value"] == updated
            results["checks"].append("value, project, and signing keys survive API restart")
            audit = request("GET", path + "/audit-log", token=token)
            entries = audit.get("audit_logs", audit.get("audit_log", audit.get("logs", [])))
            actions = {entry["action"] for entry in entries}
            assert {"project.created", "secret.created", "secret.updated", "envfile.imported"} <= actions, "missing audit rows"
            results["checks"].append("create/update/import audit rows persisted")

            cpu_before, _ = memory_cpu()
            begin = time.monotonic()
            time.sleep(5)
            cpu_after, rss = memory_cpu()
            results["idle"] = {"seconds": round(time.monotonic() - begin, 2), "cpu_percent_one_core": round(100 * (cpu_after - cpu_before) / (time.monotonic() - begin), 2), "rss_mib": round(rss, 2)}
            def read(_):
                begin = time.monotonic()
                value = request("GET", secret_path, token=token)["secret"]["value"]
                assert value == updated
                return (time.monotonic() - begin) * 1000
            begin = time.monotonic()
            cpu_before, _ = memory_cpu()
            with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
                timings = list(pool.map(read, range(100)))
            duration = time.monotonic() - begin
            cpu_after, rss = memory_cpu()
            timings.sort()
            results["reads"] = {"count": 100, "concurrency": 4, "seconds": round(duration, 3), "p95_ms": round(timings[94], 2), "cpu_seconds": round(cpu_after - cpu_before, 3), "rss_mib": round(rss, 2)}

            request("DELETE", secret_path, token=token, expected=204)
            assert request("GET", path + "/secrets?environment=alpha", token=token)["secrets"] == []
            deleted_audit = request("GET", path + "/audit-log", token=token)
            deleted_entries = deleted_audit.get("audit_logs", deleted_audit.get("audit_log", deleted_audit.get("logs", [])))
            assert any(entry["action"] == "secret.deleted" for entry in deleted_entries), "missing delete audit row"
            results["checks"].append("delete acknowledged, read back empty, and audited")
            logs = (temp / "server.log").read_bytes()
            assert first.encode() not in logs and updated.encode() not in logs, "test value in server log"
            if args.database == "sqlite":
                with sqlite3.connect(temp / "vault.db") as db:
                    assert db.execute("PRAGMA integrity_check").fetchone()[0] == "ok"
                    assert not db.execute("PRAGMA foreign_key_check").fetchall()
                for file in temp.glob("vault.db*"):
                    raw = file.read_bytes()
                    assert first.encode() not in raw and updated.encode() not in raw, "plaintext test value persisted"
                results["checks"].append("SQLite integrity, foreign keys, and no plaintext values in DB/WAL")
            results["checks"].append("no test values in server logs")
            print(json.dumps(results, indent=2))
        finally:
            if process and process.poll() is None:
                process.terminate()
                process.wait(timeout=15)
            if container:
                subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)


if __name__ == "__main__":
    main()
