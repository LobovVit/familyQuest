import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location("telegram_control", Path(__file__).with_name("telegram-control.py"))
control = importlib.util.module_from_spec(spec)
spec.loader.exec_module(control)


class TelegramControlTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.env = Path(self.temp.name) / "env"
        self.key = Path(self.temp.name) / "key"
        self.env.write_text("EXISTING='unchanged'\nTELEGRAM_ENABLED=false\nTELEGRAM_OUTBOX_ENABLED=false\n")
        self.key.write_text("svc_" + "x" * 40 + "\n")
        self.env.chmod(0o600)
        self.key.chmod(0o600)

    @patch.object(control, "probe", return_value={"state": "unconfigured"})
    def test_install_is_disabled_idempotent_and_cannot_replace_key(self, probe):
        for _ in range(2):
            result = control.configure("install-key", self.env, self.key)
            self.assertFalse(result["telegramEnabled"])
            self.assertFalse(result["outboxEnabled"])
            self.assertNotIn("svc_", str(result))
        self.assertIn("EXISTING='unchanged'", self.env.read_text())
        self.assertEqual(0o600, self.env.stat().st_mode & 0o777)
        before = self.env.read_text()
        self.key.write_text("svc_" + "y" * 40)
        with self.assertRaises(control.ConfigurationError):
            control.configure("install-key", self.env, self.key)
        self.assertEqual(before, self.env.read_text())

    def test_protected_input_rejects_public_permissions_and_symlinks(self):
        self.key.chmod(0o644)
        with self.assertRaises(control.ConfigurationError):
            control.protected_read(self.key)
        link = Path(self.temp.name) / "link"
        link.symlink_to(self.env)
        with self.assertRaises(control.ConfigurationError):
            control.protected_read(link)

    @patch.object(control, "probe")
    def test_activation_requires_current_transport_but_rollback_needs_no_network(self, probe):
        control.configure("install-key", self.env, self.key)
        before = self.env.read_text()
        probe.return_value = {"configured": True, "incoming": {"mode": "polling", "ready": False}}
        with self.assertRaises(control.ConfigurationError):
            control.configure("enable-ui", self.env)
        self.assertEqual(before, self.env.read_text())
        state = {"configured": True, "state": "blocked", "ready": False, "incoming": {"mode": "polling", "ready": True}, "delivery": {"mode": "disabled", "ready": False}, "reasons": ["delivery_disabled_or_stale"]}
        probe.return_value = state
        self.assertFalse(control.configure("enable-ui", self.env)["outboxEnabled"])
        with self.assertRaises(control.ConfigurationError):
            control.configure("enable-delivery", self.env)
        state.update(state="ready", ready=True, delivery={"mode": "linked", "ready": True}, reasons=[])
        self.assertTrue(control.configure("enable-delivery", self.env)["outboxEnabled"])
        probe.side_effect = AssertionError("rollback must not contact Portal")
        self.assertFalse(control.configure("disable", self.env)["telegramEnabled"])

    def test_concurrent_change_is_not_overwritten(self):
        before = self.env.read_text()
        self.env.write_text(before + "NEW=keep\n")
        with self.assertRaises(control.ConfigurationError):
            control.replace_config(self.env, before, {"TELEGRAM_ENABLED": "false"})
        self.assertIn("NEW=keep", self.env.read_text())

    @patch.object(control.urllib.request, "build_opener")
    def test_probe_rejects_foreign_scope_and_never_returns_key(self, build):
        response = MagicMock(status=200)
        build.return_value.open.return_value.__enter__.return_value = response
        valid = {"scope": "portal_telegram", "productId": "familyquest", "configured": False, "state": "unconfigured", "ready": False}
        for mutation in [{"productId": "lovit-dom"}, {"scope": "operator"}, {"ready": "false"}, {"state": "unknown"}]:
            response.read.return_value = json.dumps(valid | mutation).encode()
            with self.assertRaises(control.ConfigurationError):
                control.probe(self.key.read_text().strip())
        response.read.return_value = json.dumps(valid).encode()
        self.assertEqual(valid, control.probe(self.key.read_text().strip()))
        request = build.return_value.open.call_args.args[0]
        self.assertEqual("GET", request.get_method())
        self.assertEqual("https://lovit.tech/v1/telegram/readiness", request.full_url)
        self.assertIsNone(request.data)


if __name__ == "__main__":
    unittest.main()
