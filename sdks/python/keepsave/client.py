"""KeepSave Python SDK client."""

import json
import urllib.request
import urllib.error
from typing import Optional


class KeepSaveError(Exception):
    """Error returned by the KeepSave API."""

    def __init__(self, message: str, code: int = 0):
        super().__init__(message)
        self.code = code


class KeepSaveClient:
    """Client for the KeepSave API.

    Usage:
        client = KeepSaveClient("http://localhost:8080")
        client.login("user@example.com", "password")
        secrets = client.list_secrets("project-id", "alpha")
    """

    def __init__(self, base_url: str, token: Optional[str] = None, api_key: Optional[str] = None):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.api_key = api_key

    def _request(self, method: str, path: str, body: Optional[dict] = None) -> dict:
        url = f"{self.base_url}/api/v1{path}"
        headers = {"Content-Type": "application/json"}

        if self.api_key:
            headers["X-API-Key"] = self.api_key
        elif self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        data = json.dumps(body).encode("utf-8") if body else None
        req = urllib.request.Request(url, data=data, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req) as resp:
                if resp.status == 204:
                    return {}
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            try:
                err_body = json.loads(e.read().decode("utf-8"))
                msg = err_body.get("error", {}).get("message", str(e))
            except (json.JSONDecodeError, AttributeError):
                msg = str(e)
            raise KeepSaveError(msg, e.code) from e

    # Authentication

    def register(self, email: str, password: str) -> dict:
        """Register a new user account."""
        resp = self._request("POST", "/auth/register", {"email": email, "password": password})
        self.token = resp.get("token")
        return resp

    def login(self, email: str, password: str) -> dict:
        """Login and obtain a JWT token."""
        resp = self._request("POST", "/auth/login", {"email": email, "password": password})
        self.token = resp.get("token")
        return resp

    # Projects

    def list_projects(self) -> list:
        """List all projects."""
        resp = self._request("GET", "/projects")
        return resp.get("projects", [])

    def create_project(self, name: str, description: str = "") -> dict:
        """Create a new project."""
        resp = self._request("POST", "/projects", {"name": name, "description": description})
        return resp.get("project", {})

    def get_project(self, project_id: str) -> dict:
        """Get a project by ID."""
        resp = self._request("GET", f"/projects/{project_id}")
        return resp.get("project", {})

    def delete_project(self, project_id: str) -> None:
        """Delete a project."""
        self._request("DELETE", f"/projects/{project_id}")

    # Secrets

    def list_secrets(self, project_id: str, environment: str) -> list:
        """List secrets for a project in a given environment."""
        resp = self._request("GET", f"/projects/{project_id}/secrets?environment={environment}")
        return resp.get("secrets", [])

    def create_secret(self, project_id: str, key: str, value: str, environment: str) -> dict:
        """Create a secret."""
        resp = self._request("POST", f"/projects/{project_id}/secrets", {
            "key": key, "value": value, "environment": environment,
        })
        return resp.get("secret", {})

    def update_secret(self, project_id: str, secret_id: str, value: str) -> dict:
        """Update a secret value."""
        resp = self._request("PUT", f"/projects/{project_id}/secrets/{secret_id}", {"value": value})
        return resp.get("secret", {})

    def delete_secret(self, project_id: str, secret_id: str) -> None:
        """Delete a secret."""
        self._request("DELETE", f"/projects/{project_id}/secrets/{secret_id}")

    # Promotions

    def promote(self, project_id: str, source: str, target: str,
                override_policy: str = "skip", keys: Optional[list] = None,
                notes: str = "") -> dict:
        """Promote secrets between environments."""
        body = {
            "source_environment": source,
            "target_environment": target,
            "override_policy": override_policy,
        }
        if keys:
            body["keys"] = keys
        if notes:
            body["notes"] = notes
        resp = self._request("POST", f"/projects/{project_id}/promote", body)
        return resp.get("promotion", {})

    def promote_diff(self, project_id: str, source: str, target: str) -> list:
        """Preview promotion diff."""
        resp = self._request("POST", f"/projects/{project_id}/promote/diff", {
            "source_environment": source,
            "target_environment": target,
        })
        return resp.get("diff", [])

    # API Keys

    def list_api_keys(self) -> list:
        """List API keys."""
        resp = self._request("GET", "/api-keys")
        return resp.get("api_keys", [])

    def create_api_key(self, name: str, project_id: str, scopes: Optional[list] = None,
                       environment: Optional[str] = None) -> dict:
        """Create an API key."""
        body = {"name": name, "project_id": project_id, "scopes": scopes or ["read"]}
        if environment:
            body["environment"] = environment
        return self._request("POST", "/api-keys", body)

    def delete_api_key(self, key_id: str) -> None:
        """Delete an API key."""
        self._request("DELETE", f"/api-keys/{key_id}")

    # Key Rotation

    def rotate_keys(self, project_id: str) -> dict:
        """Rotate encryption keys for a project."""
        return self._request("POST", f"/projects/{project_id}/rotate-keys")

    # Import/Export

    def export_env(self, project_id: str, environment: str) -> str:
        """Export secrets as .env format."""
        resp = self._request("GET", f"/projects/{project_id}/env-export?environment={environment}")
        return resp.get("content", "")

    def import_env(self, project_id: str, environment: str, content: str,
                   overwrite: bool = False) -> dict:
        """Import secrets from .env content."""
        resp = self._request("POST", f"/projects/{project_id}/env-import", {
            "environment": environment, "content": content, "overwrite": overwrite,
        })
        return resp.get("result", {})
