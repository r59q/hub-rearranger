"""Execution policies are validated against the shared schema before comparison."""

import copy
import json
import subprocess
import sys
import unittest
from pathlib import Path

import yaml
from profile_policy import verify

ROOT = Path(__file__).resolve().parents[2]
CATALOG = ROOT / ".github/agent-profiles.yml"


class PolicyTests(unittest.TestCase):
    def setUp(self):
        self.catalog = yaml.safe_load(CATALOG.read_bytes())

    def content(self, value):
        return yaml.safe_dump(value).encode()

    def test_accepts_identical_policy_and_omits_prose(self):
        raw = self.content(self.catalog)

        result = verify(raw, raw, "codex-thorough")

        self.assertEqual(result["code"], "ACCEPTED")
        self.assertNotIn("name", result["profile"])
        self.assertNotIn("description", result["profile"])
        self.assertNotIn("enabled", result["profile"])

    def test_prose_and_set_order_do_not_change_execution_policy(self):
        current = copy.deepcopy(self.catalog)
        entry = current["profiles"]["codex-thorough"]
        entry["name"] = "New name"
        entry["description"] = "Different display prose"
        entry["context"]["sources"].reverse()

        result = verify(
            self.content(self.catalog), self.content(current), "codex-thorough"
        )

        self.assertEqual(result["code"], "ACCEPTED")

    def test_every_supported_material_change_fails_closed(self):
        changes = [
            ("adapter", "runner_label", "other-dedicated"),
            ("model", "id", "gpt-6-sol"),
            ("model", "reasoning_effort", "medium"),
            (
                "context",
                "sources",
                ["issue", "repository", "pull_request", "review_thread", "checks"],
            ),
            ("continuation", "review_comments", False),
        ]
        for group, key, value in changes:
            with self.subTest(group=group, key=key):
                current = copy.deepcopy(self.catalog)
                current["profiles"]["codex-thorough"][group][key] = value

                result = verify(
                    self.content(self.catalog), self.content(current), "codex-thorough"
                )

                self.assertIn(
                    result["code"], {"PROFILE_POLICY_CHANGED", "PROFILE_INVALID"}
                )

    def test_disabled_removed_and_invalid_catalogs_are_not_authority(self):
        for variant in ("disabled", "removed", "credential", "duplicate", "alias"):
            with self.subTest(variant=variant):
                current = copy.deepcopy(self.catalog)
                if variant == "disabled":
                    current["profiles"]["codex-thorough"]["enabled"] = False
                elif variant == "removed":
                    current["profiles"] = {}
                elif variant == "credential":
                    current["profiles"]["codex-thorough"]["token"] = "private-sentinel"
                raw = self.content(current)
                if variant == "duplicate":
                    raw += b"schema_version: 1\n"
                elif variant == "alias":
                    raw = b"schema_version: 1\nprofiles: &x {}\nother: *x\n"

                result = verify(self.content(self.catalog), raw, "codex-thorough")

                self.assertNotEqual(result["code"], "ACCEPTED")
                self.assertNotIn("private-sentinel", json.dumps(result))

    def test_pinned_revocation_also_blocks(self):
        pinned = copy.deepcopy(self.catalog)
        pinned["profiles"]["codex-thorough"]["enabled"] = False

        result = verify(
            self.content(pinned), self.content(self.catalog), "codex-thorough"
        )

        self.assertEqual(result["code"], "PROFILE_DISABLED")

    def test_cli_never_echoes_invalid_input(self):
        result = subprocess.run(
            [sys.executable, "-I", str(Path(__file__).with_name("profile_policy.py"))],
            input=b"private-sentinel",
            capture_output=True,
            check=True,
        )

        self.assertEqual(json.loads(result.stdout), {"code": "PROFILE_INVALID"})
        self.assertEqual(result.stderr, b"")


if __name__ == "__main__":
    unittest.main()
