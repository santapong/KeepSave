import io
import json
import unittest
import urllib.error
from unittest.mock import MagicMock, patch

from keepsave.client import KeepSaveClient, KeepSaveError


def response(data=None, status=200):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(data or {}).encode()
    return result


class SessionTests(unittest.TestCase):
    def test_account_switch_and_denial_clear_cache(self):
        client = KeepSaveClient("https://synthetic.invalid", token="old-session", max_retries=0)
        with patch("urllib.request.urlopen", side_effect=[response({"secrets": [{"value": "first"}]}), response({"token": "new-session"}), response({"secrets": []})]) as http:
            client.list_secrets("p", "alpha")
            client.list_secrets("p", "alpha")
            client.login("synthetic@example.invalid", "synthetic")
            client.list_secrets("p", "alpha")
            self.assertEqual(http.call_count, 3)
        denied = urllib.error.HTTPError("https://synthetic.invalid", 401, "denied", {}, io.BytesIO(b"not-json"))
        with patch("urllib.request.urlopen", side_effect=denied):
            with self.assertRaises(KeepSaveError):
                client.get_project("denied")
        self.assertIsNone(client.token)
        self.assertEqual(client._cache._store, {})

    def test_human_helpers_and_confirmed_logout_keep_api_key_separate(self):
        client = KeepSaveClient("https://synthetic.invalid", token="human-session", api_key="api-key", max_retries=0)
        with patch("urllib.request.urlopen", return_value=response({"sessions": []})) as http:
            self.assertEqual(client.list_sessions(), [])
            request = http.call_args.args[0]
            self.assertEqual(request.get_header("Authorization"), "Bearer human-session")
            self.assertIsNone(request.get_header("X-api-key"))
        unavailable = urllib.error.HTTPError("https://synthetic.invalid", 503, "unavailable", {}, io.BytesIO(b"{}"))
        client._cache.set("fixture", ["synthetic"])
        with patch("urllib.request.urlopen", side_effect=unavailable):
            with self.assertRaises(KeepSaveError):
                client.logout()
        self.assertEqual(client.token, "human-session")
        self.assertIsNotNone(client._cache.get("fixture"))
        with patch("urllib.request.urlopen", return_value=response(status=204)):
            client.logout()
        self.assertIsNone(client.token)
        self.assertEqual(client.api_key, "api-key")
        self.assertIsNone(client._cache.get("fixture"))

    def test_manual_api_key_switch_clears_identity_cache(self):
        client = KeepSaveClient("https://synthetic.invalid")
        client._cache.set("fixture", ["synthetic"])
        client.api_key = "different-key"
        self.assertIsNone(client._cache.get("fixture"))
