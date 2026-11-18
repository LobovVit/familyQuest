#!/usr/bin/env python3
"""Fail closed on npm advisories, with one owner-approved build-only exception."""
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess

ADVISORY = 'https://github.com/advisories/GHSA-fc5q-39xc-vjpf'
EXPIRES = datetime(2026, 11, 25, tzinfo=timezone.utc)


def approved(report, lock, now):
    if now >= EXPIRES or report.get('error'):
        return False
    issues = report.get('vulnerabilities')
    if not isinstance(issues, dict) or not issues:
        return False

    def allowed(name, visiting):
        if name in visiting or name not in issues:
            return False
        issue = issues[name]
        nodes = issue.get('nodes', [])
        if not nodes or not all(lock.get('packages', {}).get(node, {}).get('dev') is True for node in nodes):
            return False
        reasons = issue.get('via', [])
        if not reasons:
            return False
        for reason in reasons:
            if isinstance(reason, str):
                if not allowed(reason, visiting | {name}):
                    return False
            elif not isinstance(reason, dict) or name != 'picomatch' or reason.get('name') != 'picomatch' or reason.get('url') != ADVISORY:
                return False
        return True

    return all(allowed(name, set()) for name in issues)


def main():
    root = Path(__file__).resolve().parents[1] / 'frontend'
    result = subprocess.run(['npm', 'audit', '--json'], cwd=root, capture_output=True, text=True)
    try:
        report = json.loads(result.stdout)
        lock = json.loads((root / 'package-lock.json').read_text())
    except (ValueError, OSError):
        raise SystemExit('npm audit unavailable or malformed; release blocked')
    if result.returncode == 0 and not report.get('error'):
        print('npm audit passed without exceptions')
        return
    if result.returncode == 1 and approved(report, lock, datetime.now(timezone.utc)):
        print('Owner-approved build-only exception: GHSA-fc5q-39xc-vjpf; expires 2026-11-25 00:00 UTC. No other advisory is allowed.')
        return
    print(result.stdout)
    raise SystemExit('npm audit failed: new/runtime advisory, unavailable report, or expired exception')


if __name__ == '__main__':
    main()
