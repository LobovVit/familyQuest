#!/usr/bin/env python3
"""Prepare or change FamilyQuest Telegram flags; never issue keys or send messages.

Run on the production host from /opt/familyQuest. Enabling requires the separately
recorded owner approval in deploy/telegram-production.json. Recreate Access after
a successful change. The key and existing configuration are never printed.
"""
import argparse
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import urllib.error
import urllib.request

ORIGIN = "https://lovit.tech"
KEY = re.compile(r"[A-Za-z0-9_-]{32,256}\Z")
NAMES = {"TELEGRAM_ENABLED", "TELEGRAM_OUTBOX_ENABLED", "TELEGRAM_PORTAL_ORIGIN", "TELEGRAM_SERVICE_KEY"}


class ConfigurationError(Exception):
    pass


def protected_read(path):
    # O_NOFOLLOW also refuses a symlink handoff. The file must belong to this operator.
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd, "r", encoding="utf-8") as source:
            info = os.fstat(source.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600:
                raise ConfigurationError("protected_file_requires_owner_and_mode_0600")
            data = source.read(131073)
            if len(data) > 131072:
                raise ConfigurationError("protected_file_too_large")
            return data
    except OSError:
        raise ConfigurationError("protected_file_unavailable") from None


def values(text):
    result = {}
    for line in text.splitlines():
        match = re.fullmatch(r"\s*(?:export\s+)?(TELEGRAM_[A-Z_]+)\s*=\s*(.*?)\s*", line)
        if not match or match[1] not in NAMES:
            continue
        if match[1] in result:
            raise ConfigurationError("duplicate_telegram_configuration")
        value = match[2]
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        result[match[1]] = value
    return result


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def probe(key):
    if not KEY.fullmatch(key):
        raise ConfigurationError("invalid_product_key")
    req = urllib.request.Request(ORIGIN + "/v1/telegram/readiness", headers={"Authorization": "Bearer " + key})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        with opener.open(req, timeout=6) as response:
            if response.status != 200:
                raise ConfigurationError("portal_readiness_unavailable")
            raw = response.read(65537)
            if len(raw) > 65536:
                raise ConfigurationError("portal_readiness_invalid")
            state = json.loads(raw)
    except (OSError, ValueError, urllib.error.URLError):
        raise ConfigurationError("portal_readiness_unavailable") from None
    if not isinstance(state, dict) or state.get("scope") != "portal_telegram" or state.get("productId") != "familyquest" or state.get("state") not in {"disabled", "unconfigured", "pilot", "blocked", "ready"} or type(state.get("ready")) is not bool or type(state.get("configured")) is not bool:
        raise ConfigurationError("portal_product_or_contract_mismatch")
    return state


def replace_config(path, original, changes):
    lines = []
    for line in original.splitlines():
        match = re.match(r"\s*(?:export\s+)?(TELEGRAM_[A-Z_]+)\s*=", line)
        if not match or match[1] not in changes:
            lines.append(line)
    lines.extend(name + "='" + value + "'" for name, value in changes.items())
    if protected_read(path) != original:
        raise ConfigurationError("configuration_changed_during_preflight")
    fd, temporary = tempfile.mkstemp(prefix=".telegram-env-", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            output.write("\n".join(lines) + "\n")
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def configure(action, env_path, key_path=None):
    original = protected_read(env_path)
    current = values(original)
    changes = {"TELEGRAM_ENABLED": "false", "TELEGRAM_OUTBOX_ENABLED": "false"}
    if action == "install-key":
        if current.get("TELEGRAM_ENABLED", "false") != "false" or current.get("TELEGRAM_OUTBOX_ENABLED", "false") != "false":
            raise ConfigurationError("disable_before_key_installation")
        key = protected_read(key_path).strip()
        if current.get("TELEGRAM_SERVICE_KEY", "") not in {"", key}:
            raise ConfigurationError("existing_key_must_not_be_replaced")
        probe(key)
        changes.update(TELEGRAM_PORTAL_ORIGIN=ORIGIN, TELEGRAM_SERVICE_KEY=key)
    elif action != "disable":
        if current.get("TELEGRAM_PORTAL_ORIGIN") != ORIGIN:
            raise ConfigurationError("portal_origin_not_installed")
        state = probe(current.get("TELEGRAM_SERVICE_KEY", ""))
        incoming, delivery = state.get("incoming"), state.get("delivery")
        if not state["configured"] or state.get("state") not in {"blocked", "pilot", "ready"} or not isinstance(incoming, dict) or incoming.get("ready") is not True or incoming.get("mode") not in {"polling", "webhook"}:
            raise ConfigurationError("incoming_transport_not_ready")
        if action == "enable-delivery":
            if state["state"] != "ready" or state["ready"] is not True or not isinstance(delivery, dict) or delivery.get("ready") is not True or delivery.get("mode") != "linked" or state.get("reasons") != []:
                raise ConfigurationError("delivery_transport_not_ready")
            changes["TELEGRAM_OUTBOX_ENABLED"] = "true"
        changes["TELEGRAM_ENABLED"] = "true"
    replace_config(env_path, original, changes)
    return {"project": "familyquest", "action": action, "configurationWritten": True,
            "telegramEnabled": changes["TELEGRAM_ENABLED"] == "true",
            "outboxEnabled": changes["TELEGRAM_OUTBOX_ENABLED"] == "true",
            "runtimeChanged": False, "nextAction": "Recreate Access using the approved release images"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["install-key", "enable-ui", "enable-delivery", "disable"])
    parser.add_argument("--env-file", type=Path, default=Path("/opt/familyQuest/.env"))
    parser.add_argument("--key-file", type=Path, default=Path("/opt/familyQuest/private/telegram-service-key"))
    args = parser.parse_args()
    try:
        print(json.dumps(configure(args.action, args.env_file, args.key_file)))
    except ConfigurationError as error:
        print(json.dumps({"ok": False, "code": str(error)}))
        raise SystemExit(2) from None
    except OSError:
        print(json.dumps({"ok": False, "code": "configuration_write_failed"}))
        raise SystemExit(2) from None


if __name__ == "__main__":
    main()
