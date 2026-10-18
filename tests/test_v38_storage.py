from __future__ import annotations

import json
import os
import shutil
import sys
import tempfile
import unittest
from unittest import mock
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
import test_server  # noqa: E402


class SQLiteMigrationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        shutil.copytree(ROOT / "resources", self.root / "resources")
        shutil.copytree(ROOT / "data", self.root / "data")
        self.legacy_path = self.root / "data" / "account_state.json"
        self.db_path = self.root / "data" / "sigma.db"
        self.profiles_path = self.root / "resources" / "source_default_profiles.json"
        self.maps_path = self.root / "resources" / "available_maps.json"
        for sidecar in (self.db_path, Path(str(self.db_path) + "-wal"), Path(str(self.db_path) + "-shm")):
            sidecar.unlink(missing_ok=True)

    def tearDown(self) -> None:
        self.temp.cleanup()

    def open_store(self) -> test_server.SQLiteAccountStore:
        return test_server.SQLiteAccountStore(
            self.db_path, self.legacy_path, self.profiles_path, self.maps_path
        )

    def test_migrates_entire_legacy_account_and_seeds_maps(self) -> None:
        before = self.legacy_path.read_bytes()
        store = self.open_store()
        self.assertEqual(store.state["account_id"], 100000001)
        self.assertEqual(len(store.state["profile"]["profiles"]), 51)
        self.assertEqual(store.maps_payload()["selected_map"], "PARADISE")
        self.assertEqual(
            [row["map_code"] for row in store.available_maps()],
            ["PARADISE", "AZORES", "EMAPSONORAN", "PARADISE_NEO", "PARADISE_NEOMAX"],
        )
        self.assertEqual(self.legacy_path.read_bytes(), before, "legacy JSON must remain a backup")
        self.assertTrue(self.db_path.exists())
        self.assertEqual(self.db_path.stat().st_mode & 0o777, 0o600)

    def test_imports_existing_maps_config_and_keeps_changes_after_restart(self) -> None:
        legacy = json.loads(self.legacy_path.read_text(encoding="utf-8"))
        legacy["maps_config"] = {
            "selected_map": "AZORES",
            "selected_model": "BATTLEROYALE",
            "unlocked_maps": ["AZORES", "PARADISE_NEO"],
        }
        self.legacy_path.write_text(json.dumps(legacy), encoding="utf-8")
        store = self.open_store()
        self.assertEqual(store.maps_payload()["selected_map"], "AZORES")
        self.assertEqual(
            [row["map_code"] for row in store.available_maps()],
            ["AZORES", "PARADISE_NEO"],
        )
        store.state["exp"] = 876543
        store.save()
        reopened = self.open_store()
        self.assertEqual(reopened.state["exp"], 876543)
        self.assertEqual(reopened.maps_payload()["selected_map"], "AZORES")

    def test_map_selection_only_accepts_enabled_catalog_entries(self) -> None:
        store = self.open_store()
        self.assertFalse(store.select_map("UNKNOWN_MAP"))
        self.assertTrue(store.select_map("PARADISE_NEOMAX"))
        reopened = self.open_store()
        self.assertEqual(reopened.maps_payload()["selected_map"], "PARADISE_NEOMAX")


class BillboardResponseTests(unittest.TestCase):
    def test_login_broadcast_contains_configured_lobby_marquee(self) -> None:
        account = {"account_id": 100000001, "region": "IND"}
        with mock.patch.dict(os.environ, {
            "SIGMA_BILLBOARD_TEXT": "WELCOME TO SIGMA PRIVATE SERVER!",
            "SIGMA_BILLBOARD_LANGUAGE": "en",
        }, clear=False):
            response = test_server.login_broadcast_response(account)
        self.assertIn(b"WELCOME TO SIGMA PRIVATE SERVER!", response)
        self.assertIn(b"IND", response)
        # Outer field 1 is LoginBroadcastRes.scroll_res; the inner field 1 is
        # CSScrollMarqueeRes.scrollMarquees. Both are length-delimited messages.
        self.assertEqual(response[0] & 0x07, 2)
        self.assertEqual(response[2] & 0x07, 2)

    def test_dedicated_scroll_marquee_returns_same_repeatable_ticker(self) -> None:
        account = {"account_id": 100000001, "region": "IND"}
        with mock.patch.dict(os.environ, {
            "SIGMA_BILLBOARD_TEXT": "WELCOME BACK TO SIGMA!",
            "SIGMA_BILLBOARD_LANGUAGE": "en",
        }, clear=False):
            response = test_server.scroll_marquee_response(account)
        self.assertIn(b"WELCOME BACK TO SIGMA!", response)
        self.assertIn(b"IND", response)
        self.assertEqual(response[0] & 0x07, 2)  # CSScrollMarqueeRes.scrollMarquees

    def test_marquee_refresh_is_fresh_and_active_with_clock_skew_grace(self) -> None:
        account = {"account_id": 100000001, "region": "IND"}
        with mock.patch.dict(os.environ, {
            "SIGMA_BILLBOARD_TEXT": "ALWAYS HERE",
            "SIGMA_BILLBOARD_LANGUAGE": "en",
            "SIGMA_BILLBOARD_START_GRACE_SECONDS": "3600",
        }, clear=False), mock.patch.object(test_server.time, "time", side_effect=[10000, 10001]):
            first = test_server.login_broadcast_response(account)
            refreshed = test_server.scroll_marquee_response(account)
        self.assertIn(b"ALWAYS HERE", first)
        self.assertIn(b"ALWAYS HERE", refreshed)
        # The nested descriptor's start timestamp is within the past grace window.
        self.assertIn(test_server.pb_varint_field(4, 6400), first)
        self.assertIn(test_server.pb_varint_field(4, 6401), refreshed)
        self.assertNotEqual(first, refreshed)

    def test_empty_billboard_text_returns_empty_declared_response(self) -> None:
        with mock.patch.dict(os.environ, {"SIGMA_BILLBOARD_TEXT": "   "}, clear=False):
            self.assertEqual(test_server.login_broadcast_response({"region": "IND"}), b"")
            self.assertEqual(test_server.scroll_marquee_response({"region": "IND"}), b"")


class TCPAdvertisementTests(unittest.TestCase):
    def test_login_data_points_notification_channel_to_matchmaking_tcp(self) -> None:
        account = {"account_id": 100000001, "region": "IND", "display_name": "LOCAL_SIGMA_TEST"}
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("SIGMA_NOTIFICATION_CHANNEL", None)
            response = test_server.login_data_response(account)
        self.assertIn(test_server.pb_string_field(14, "127.0.0.1:10300"), response)

    def test_version_response_uses_configured_match_host(self) -> None:
        handler = object.__new__(test_server.Handler)
        handler.path = "/?version=1.0.0"
        handler.base_url = lambda: "http://127.0.0.1:3000"
        handler.server = type("Server", (), {"state": type("State", (), {"account": {"region": "IND"}})()})()
        with mock.patch.dict(os.environ, {"SIGMA_MATCH_HOST": "192.168.1.25", "SIGMA_GAME_NODE_PORT": "10300"}, clear=False):
            response = json.loads(handler.version_response())
        self.assertEqual(response["tcp_server"], "http://192.168.1.25:10300")


if __name__ == "__main__":
    unittest.main()
