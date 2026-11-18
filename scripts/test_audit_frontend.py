from copy import deepcopy
from datetime import datetime, timezone
import unittest

from audit_frontend import ADVISORY, approved


class FrontendAuditPolicy(unittest.TestCase):
    def setUp(self):
        self.now = datetime(2026, 11, 18, tzinfo=timezone.utc)
        self.report = {'vulnerabilities': {
            'picomatch': {'nodes': ['node_modules/picomatch'], 'via': [{'name': 'picomatch', 'url': ADVISORY}]},
            'vite': {'nodes': ['node_modules/vite'], 'via': ['picomatch']},
        }}
        self.lock = {'packages': {'node_modules/picomatch': {'dev': True}, 'node_modules/vite': {'dev': True}}}

    def test_only_approved_build_dependency_passes(self):
        self.assertTrue(approved(self.report, self.lock, self.now))

    def test_expiry_is_enforced(self):
        self.assertFalse(approved(self.report, self.lock, datetime(2026, 11, 25, tzinfo=timezone.utc)))

    def test_runtime_dependency_is_never_excepted(self):
        lock = deepcopy(self.lock)
        lock['packages']['node_modules/picomatch']['dev'] = False
        self.assertFalse(approved(self.report, lock, self.now))

    def test_other_advisory_and_mixed_advisory_fail(self):
        for replace in (True, False):
            report = deepcopy(self.report)
            reason = {'name': 'picomatch', 'url': 'https://github.com/advisories/another-advisory'}
            if replace:
                report['vulnerabilities']['picomatch']['via'] = [reason]
            else:
                report['vulnerabilities']['picomatch']['via'].append(reason)
            self.assertFalse(approved(report, self.lock, self.now))

    def test_unavailable_or_cycle_fails(self):
        self.assertFalse(approved({'error': 'unavailable'}, self.lock, self.now))
        report = deepcopy(self.report)
        report['vulnerabilities']['picomatch']['via'] = ['vite']
        self.assertFalse(approved(report, self.lock, self.now))
