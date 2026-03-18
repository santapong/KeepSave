"""KeepSave Python SDK v2.0.0 client.

Features:
    - Full CRUD for projects, secrets, promotions, and API keys
    - Automatic retry with exponential backoff
    - Circuit breaker for API resilience
    - Batch secret fetch
    - In-memory cache with TTL
    - Automatic secret refresh on rotation detection
"""

import json
import time
import threading
import urllib.request
import urllib.error
from typing import Any, Dict, List, Optional


class KeepSaveError(Exception):
    """Error returned by the KeepSave API."""

    def __init__(self, message: str, code: int = 0):
        super().__init__(message)
        self.code = code


class CircuitBreakerOpenError(KeepSaveError):
    """Raised when the circuit breaker is open."""

    def __init__(self) -> None:
        super().__init__("Circuit breaker is open — requests are temporarily blocked", 503)


# ── Circuit Breaker ──────────────────────────────────────────────────


class _CircuitBreaker:
    CLOSED = "CLOSED"
    OPEN = "OPEN"
    HALF_OPEN = "HALF_OPEN"

    def __init__(self, threshold: int = 5, reset_timeout: float = 30.0):
        self._state = self.CLOSED
        self._failure_count = 0
        self._last_failure_time = 0.0
        self._threshold = threshold
        self._reset_timeout = reset_timeout
        self._lock = threading.Lock()

    @property
    def state(self) -> str:
        with self._lock:
            return self._state

    def can_execute(self) -> bool:
        with self._lock:
            if self._state == self.CLOSED:
                return True
            if self._state == self.OPEN:
                if time.monotonic() - self._last_failure_time >= self._reset_timeout:
                    self._state = self.HALF_OPEN
                    return True
                return False
            return True  # HALF_OPEN

    def on_success(self) -> None:
        with self._lock:
            self._failure_count = 0
            self._state = self.CLOSED

    def on_failure(self) -> None:
        with self._lock:
            self._failure_count += 1
            self._last_failure_time = time.monotonic()
            if self._failure_count >= self._threshold:
                self._state = self.OPEN


# ── Cache ────────────────────────────────────────────────────────────


class _Cache:
    def __init__(self, ttl: float = 60.0):
        self._store: Dict[str, Any] = {}
        self._expiry: Dict[str, float] = {}
        self._ttl = ttl
        self._lock = threading.Lock()

    def get(self, key: str) -> Any:
        with self._lock:
            if key not in self._store:
                return None
            if time.monotonic() > self._expiry[key]:
                del self._store[key]
                del self._expiry[key]
                return None
            return self._store[key]

    def set(self, key: str, value: Any) -> None:
        if self._ttl <= 0:
            return
        with self._lock:
            self._store[key] = value
            self._expiry[key] = time.monotonic() + self._ttl

    def invalidate(self, prefix: str) -> None:
        with self._lock:
            keys_to_delete = [k for k in self._store if k.startswith(prefix)]
            for k in keys_to_delete:
                del self._store[k]
                del self._expiry[k]

    def clear(self) -> None:
        with self._lock:
            self._store.clear()
            self._expiry.clear()


# ── Client ───────────────────────────────────────────────────────────


class KeepSaveClient:
    """Client for the KeepSave API.

    Usage:
        client = KeepSaveClient("http://localhost:8080")
        client.login("user@example.com", "password")
        secrets = client.list_secrets("project-id", "alpha")

    Advanced:
        client = KeepSaveClient(
            "http://localhost:8080",
            max_retries=3,
            circuit_breaker=True,
            cache_ttl=120.0,
        )
    """

    def __init__(
        self,
        base_url: str,
        token: Optional[str] = None,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        max_retries: int = 3,
        circuit_breaker: bool = True,
        circuit_breaker_threshold: int = 5,
        circuit_breaker_reset_timeout: float = 30.0,
        cache_ttl: float = 60.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.api_key = api_key
        self._timeout = timeout
        self._max_retries = max_retries
        self._cache = _Cache(ttl=cache_ttl)
        self._breaker: Optional[_CircuitBreaker] = None
        if circuit_breaker:
            self._breaker = _CircuitBreaker(
                threshold=circuit_breaker_threshold,
                reset_timeout=circuit_breaker_reset_timeout,
            )

    def clear_cache(self) -> None:
        """Clear the local secret cache."""
        self._cache.clear()

    @property
    def circuit_state(self) -> str:
        """Get circuit breaker state (CLOSED / OPEN / HALF_OPEN / DISABLED)."""
        return self._breaker.state if self._breaker else "DISABLED"

    # ── Internal HTTP ────────────────────────────────────────────────

    def _request(self, method: str, path: str, body: Optional[dict] = None) -> dict:
        if self._breaker and not self._breaker.can_execute():
            raise CircuitBreakerOpenError()

        last_error: Optional[Exception] = None

        for attempt in range(self._max_retries + 1):
            try:
                result = self._do_request(method, path, body)
                if self._breaker:
                    self._breaker.on_success()
                return result
            except Exception as e:
                last_error = e
                if not self._is_retryable(e) or attempt == self._max_retries:
                    if self._breaker:
                        self._breaker.on_failure()
                    raise
                delay = min(0.2 * (2 ** attempt), 5.0)
                time.sleep(delay)

        raise last_error  # type: ignore[misc]

    def _do_request(self, method: str, path: str, body: Optional[dict] = None) -> dict:
        url = f"{self.base_url}/api/v1{path}"
        headers = {"Content-Type": "application/json"}

        if self.api_key:
            headers["X-API-Key"] = self.api_key
        elif self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        data = json.dumps(body).encode("utf-8") if body else None
        req = urllib.request.Request(url, data=data, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                if resp.status == 204:
                    return {}
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            try:
                err_body = json.loads(e.read().decode("utf-8"))
                msg = err_body.get("error", {})
                if isinstance(msg, dict):
                    msg = msg.get("message", str(e))
            except (json.JSONDecodeError, AttributeError):
                msg = str(e)
            raise KeepSaveError(msg, e.code) from e

    @staticmethod
    def _is_retryable(err: Exception) -> bool:
        if isinstance(err, KeepSaveError):
            return err.code >= 500 or err.code == 429
        if isinstance(err, (urllib.error.URLError, TimeoutError, OSError)):
            return True
        return False

    # ── Authentication ───────────────────────────────────────────────

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

    # ── Projects ─────────────────────────────────────────────────────

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
        self._cache.invalidate(f"secrets:{project_id}")
        self._request("DELETE", f"/projects/{project_id}")

    # ── Secrets ──────────────────────────────────────────────────────

    def list_secrets(self, project_id: str, environment: str) -> list:
        """List secrets for a project in a given environment. Results are cached."""
        cache_key = f"secrets:{project_id}:{environment}"
        cached = self._cache.get(cache_key)
        if cached is not None:
            return cached

        resp = self._request("GET", f"/projects/{project_id}/secrets?environment={environment}")
        secrets = resp.get("secrets", [])
        self._cache.set(cache_key, secrets)
        return secrets

    def create_secret(self, project_id: str, key: str, value: str, environment: str) -> dict:
        """Create a secret."""
        self._cache.invalidate(f"secrets:{project_id}:{environment}")
        resp = self._request("POST", f"/projects/{project_id}/secrets", {
            "key": key, "value": value, "environment": environment,
        })
        return resp.get("secret", {})

    def update_secret(self, project_id: str, secret_id: str, value: str) -> dict:
        """Update a secret value."""
        self._cache.invalidate(f"secrets:{project_id}")
        resp = self._request("PUT", f"/projects/{project_id}/secrets/{secret_id}", {"value": value})
        return resp.get("secret", {})

    def delete_secret(self, project_id: str, secret_id: str) -> None:
        """Delete a secret."""
        self._cache.invalidate(f"secrets:{project_id}")
        self._request("DELETE", f"/projects/{project_id}/secrets/{secret_id}")

    def batch_get_secrets(self, project_id: str, environment: str, keys: List[str]) -> dict:
        """Batch fetch multiple secrets by key name in a single request.

        Returns dict with 'secrets' list and optional 'missing_keys' list.
        """
        return self._request("POST", f"/projects/{project_id}/secrets/batch", {
            "environment": environment, "keys": keys,
        })

    def refresh_secrets(self, project_id: str, environment: str) -> list:
        """Re-fetch secrets from server, bypassing cache. Useful after key rotation."""
        self._cache.invalidate(f"secrets:{project_id}:{environment}")
        return self.list_secrets(project_id, environment)

    # ── Promotions ───────────────────────────────────────────────────

    def promote(self, project_id: str, source: str, target: str,
                override_policy: str = "skip", keys: Optional[list] = None,
                notes: str = "") -> dict:
        """Promote secrets between environments."""
        body: Dict[str, Any] = {
            "source_environment": source,
            "target_environment": target,
            "override_policy": override_policy,
        }
        if keys:
            body["keys"] = keys
        if notes:
            body["notes"] = notes
        self._cache.invalidate(f"secrets:{project_id}:{target}")
        resp = self._request("POST", f"/projects/{project_id}/promote", body)
        return resp.get("promotion", {})

    def promote_diff(self, project_id: str, source: str, target: str) -> list:
        """Preview promotion diff."""
        resp = self._request("POST", f"/projects/{project_id}/promote/diff", {
            "source_environment": source,
            "target_environment": target,
        })
        return resp.get("diff", [])

    # ── API Keys ─────────────────────────────────────────────────────

    def list_api_keys(self) -> list:
        """List API keys."""
        resp = self._request("GET", "/api-keys")
        return resp.get("api_keys", [])

    def create_api_key(self, name: str, project_id: str, scopes: Optional[list] = None,
                       environment: Optional[str] = None) -> dict:
        """Create an API key."""
        body: Dict[str, Any] = {"name": name, "project_id": project_id, "scopes": scopes or ["read"]}
        if environment:
            body["environment"] = environment
        return self._request("POST", "/api-keys", body)

    def delete_api_key(self, key_id: str) -> None:
        """Delete an API key."""
        self._request("DELETE", f"/api-keys/{key_id}")

    # ── Key Rotation ─────────────────────────────────────────────────

    def rotate_keys(self, project_id: str) -> dict:
        """Rotate encryption keys for a project. Invalidates local cache."""
        self._cache.invalidate(f"secrets:{project_id}")
        return self._request("POST", f"/projects/{project_id}/rotate-keys")

    # ── Import/Export ────────────────────────────────────────────────

    def export_env(self, project_id: str, environment: str) -> str:
        """Export secrets as .env format."""
        resp = self._request("GET", f"/projects/{project_id}/env-export?environment={environment}")
        return resp.get("content", "")

    def import_env(self, project_id: str, environment: str, content: str,
                   overwrite: bool = False) -> dict:
        """Import secrets from .env content."""
        self._cache.invalidate(f"secrets:{project_id}:{environment}")
        resp = self._request("POST", f"/projects/{project_id}/env-import", {
            "environment": environment, "content": content, "overwrite": overwrite,
        })
        return resp.get("result", {})

    # ── Application Dashboard ───────────────────────────────────────

    def list_applications(self, search: str = "", category: str = "",
                          limit: int = 50, offset: int = 0) -> dict:
        """List applications with optional search, category filter, and pagination.

        Returns dict with 'applications', 'categories', 'total', 'limit', 'offset'.
        """
        params = []
        if search:
            params.append(f"search={search}")
        if category and category != "All":
            params.append(f"category={category}")
        params.append(f"limit={limit}")
        params.append(f"offset={offset}")
        query = "&".join(params)
        return self._request("GET", f"/applications?{query}")

    def create_application(self, name: str, url: str, description: str = "",
                           icon: str = "🚀", category: str = "General") -> dict:
        """Create a new application."""
        resp = self._request("POST", "/applications", {
            "name": name, "url": url, "description": description,
            "icon": icon, "category": category,
        })
        return resp.get("application", {})

    def get_application(self, app_id: str) -> dict:
        """Get a single application by ID."""
        resp = self._request("GET", f"/applications/{app_id}")
        return resp.get("application", {})

    def update_application(self, app_id: str, name: str, url: str,
                           description: str = "", icon: str = "",
                           category: str = "") -> dict:
        """Update an application."""
        resp = self._request("PUT", f"/applications/{app_id}", {
            "name": name, "url": url, "description": description,
            "icon": icon, "category": category,
        })
        return resp.get("application", {})

    def delete_application(self, app_id: str) -> None:
        """Delete an application."""
        self._request("DELETE", f"/applications/{app_id}")

    def toggle_application_favorite(self, app_id: str) -> bool:
        """Toggle favorite status for an application. Returns new is_favorite state."""
        resp = self._request("POST", f"/applications/{app_id}/favorite")
        return resp.get("is_favorite", False)
