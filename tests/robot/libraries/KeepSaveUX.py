"""Synthetic, loopback-only fixtures for the Robot UX acceptance slice.

Credentials are read internally and never returned to Robot variables or logs.
This fixture refuses remote services and never edits an existing project.
"""
import json
import os
import time
import uuid
from pathlib import Path
from urllib.parse import urlsplit

import requests
from robot.api.deco import keyword, library
from robot.api.types import Secret
from robot.libraries.BuiltIn import BuiltIn


def validate_fixture(data, origin):
    target = urlsplit(origin)
    if (target.scheme != "http" or target.hostname != "127.0.0.1"
            or not target.port or target.path not in ("", "/")
            or target.query or target.fragment or target.username
            or data.get("origin") != origin or data.get("sample_data") is not True):
        raise ValueError("Only an explicitly labeled synthetic fixture at the exact loopback origin is allowed")


@library(scope="SUITE", auto_keywords=False)
class KeepSaveUX:
    def __init__(self):
        self.origin = os.environ.get("KEEPSAVE_UI_URL", "http://127.0.0.1:4663")
        self.fixture_path = os.environ.get("KEEPSAVE_UX_FIXTURE")
        self.token = None
        self.project = None
        self.http = requests.Session()
        self.http.trust_env = False
        self.timings = []

    def _fixture(self):
        if not self.fixture_path:
            raise ValueError("KEEPSAVE_UX_FIXTURE must identify a private synthetic context file")
        data = json.loads(Path(self.fixture_path).read_text())
        validate_fixture(data, self.origin)
        return data

    def _request(self, method, path, expected=200, body=None, token=None):
        self._fixture()  # Validate destination before every protected action.
        start = time.monotonic()
        response = self.http.request(method, self.origin + "/api/v1" + path,
            json=body, headers={"Authorization": "Bearer " + token} if token else {},
            timeout=(3, 15), allow_redirects=False)
        self.timings.append({"method": method, "status": response.status_code,
                             "milliseconds": round((time.monotonic()-start)*1000, 2),
                             "bytes": len(response.content)})
        if response.status_code != expected:
            raise AssertionError(f"{method} operation returned HTTP{response.status_code}; expected{expected}")
        if len(response.content) > 1024*1024:
            raise AssertionError("Fixture response exceeded1MiB")
        return response.json() if response.content else None

    def _login(self):
        data = self._fixture()
        return self._request("POST", "/auth/login", body={"email": data["email"], "password": data["password"]})["token"]

    @keyword
    def prepare_synthetic_project(self):
        self.token = self._login()
        self.project = self._request("POST", "/projects", 201,
            {"name": "Robot UX " + uuid.uuid4().hex[:10], "description": "Owned disposable Robot acceptance fixture"}, self.token)["project"]["id"]

    @keyword
    def open_synthetic_project(self):
        browser = BuiltIn().get_library_instance("Browser")
        browser.go_to(self.origin + "/projects/" + self.project)
        browser.wait_for_elements_state('button:has-text("Add Secret")')

    @keyword
    def sign_in_to_synthetic_workspace(self):
        data = self._fixture()
        browser = BuiltIn().get_library_instance("Browser")
        previous = BuiltIn().set_log_level("NONE")
        try:
            browser.go_to(self.origin + "/login")
            browser.fill_text('input[type="email"]', data["email"])
            browser.fill_secret('input[type="password"]', Secret(data["password"]))
            browser.click('button:has-text("Enter the vault")')
            browser.wait_for_elements_state('h1:has-text("Projects")')
        finally:
            BuiltIn().set_log_level(previous)

    @keyword
    def clean_up_synthetic_project(self):
        try:
            if self.project and self.token:
                self._request("DELETE", "/projects/" + self.project, expected=204, token=self.token)
                self.project = None
        finally:
            output = Path(BuiltIn().get_variable_value("${OUTPUT DIR}"))
            output.mkdir(parents=True, exist_ok=True)
            (output / ("api-timings-" + uuid.uuid4().hex[:6] + ".json")).write_text(json.dumps(self.timings, indent=2))
            self.http.close()

    @keyword
    def unauthenticated_reads_are_denied(self):
        self._request("GET", "/projects", expected=401)

    @keyword
    def batch_reads_preserve_environment_and_missing_keys(self):
        base = "/projects/" + self.project + "/secrets"
        self._request("POST", base, 201, {"key":"ROBOT_API_MARKER", "value":"synthetic-only", "environment":"alpha"}, self.token)
        alpha = self._request("POST", base+"/batch", body={"environment":"alpha", "keys":["ROBOT_API_MARKER", "MISSING"]}, token=self.token)
        if (len(alpha["secrets"]) != 1 or alpha["secrets"][0].get("key") != "ROBOT_API_MARKER"
                or alpha["secrets"][0].get("value") != "synthetic-only" or alpha["missing_keys"] != ["MISSING"]):
            raise AssertionError("Alpha batch result or missing-key contract differed")
        uat = self._request("POST", base+"/batch", body={"environment":"uat", "keys":["ROBOT_API_MARKER"]}, token=self.token)
        if uat["secrets"] or uat["missing_keys"] != ["ROBOT_API_MARKER"]:
            raise AssertionError("Alpha marker crossed into UAT")
        self._request("POST", base+"/batch", 400, {"environment":"alpha", "keys":[]}, self.token)

    @keyword
    def revoked_session_cannot_read_projects(self):
        owned_session = self._login()
        self._request("POST", "/auth/logout", expected=204, token=owned_session)
        self._request("GET", "/projects", 401, token=owned_session)

    @keyword
    def serial_read_probe(self):
        """Twelve serial reads: a bounded observation, not a throughput benchmark."""
        for _ in range(12):
            self._request("GET", "/projects/"+self.project+"/secrets?environment=alpha", token=self.token)

    @keyword
    def remote_fixture_is_rejected_before_network_access(self):
        try:
            validate_fixture({"origin":"https://example.invalid", "sample_data":True}, "https://example.invalid")
        except ValueError:
            return
        raise AssertionError("Remote fixture was accepted")
