"""Offline evidence/producer tests; no account, network, or real runner is used."""

import datetime
import hashlib
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import jsonschema
import yaml

from profile_diagnostic import main, probe, record

ROOT = Path(__file__).resolve().parents[2]
IDENTITY = {
    "GITHUB_REPOSITORY": "octo/demo",
    "GITHUB_SHA": "a" * 40,
    "GITHUB_RUN_ID": "10",
    "GITHUB_RUN_ATTEMPT": "2",
}


class ProfileDiagnosticTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.binary = self.root / "codex"
        self.trace = self.root / "calls"
        schema = json.loads(
            Path(__file__).with_name("readiness.schema.v1.json").read_text()
        )
        self.validator = jsonschema.Draft202012Validator(
            schema, format_checker=jsonschema.FormatChecker()
        )

    def fake(self, failure=""):
        self.binary.write_text(
            "#!/usr/bin/python3\nimport json, os, sys\n"
            f"failure = {failure!r}\n"
            f"with open({str(self.trace)!r}, 'a') as output:\n"
            "    output.write(json.dumps({'args':sys.argv[1:], 'env':dict(os.environ), "
            "'cwd':os.getcwd()}) + '\\n')\n"
            "if '--version' in sys.argv:\n"
            "    print('RAW_PRIVATE' if failure == 'version' "
            "else 'codex-cli 0.157.1')\n"
            "elif 'login' in sys.argv:\n"
            "    print('RAW_PRIVATE' if failure == 'auth' "
            "else 'Logged in using ChatGPT')\n"
            "else:\n"
            "    print('RAW_PRIVATE' if failure == 'response' else 'HUB_CODEX_READY')\n"
            "    print('model: ' + ('other' if failure == 'model' else 'gpt-6.1-sol'), "
            "file=sys.stderr)\n"
            "    print('reasoning effort: ' + "
            "('low' if failure == 'effort' else 'high'), "
            "file=sys.stderr)\n"
            "    if failure == 'ambiguous': "
            "print('model: gpt-6.1-sol', file=sys.stderr)\n"
            "    if failure == 'rerouted': "
            "print('model rerouted: other', file=sys.stderr)\n"
            "    if failure == 'warning': print('warning: changed', file=sys.stderr)\n"
            "    print('RAW_PRIVATE', file=sys.stderr)\n"
        )
        self.binary.chmod(0o700)

    def test_success_proves_effective_policy_and_strips_credentials(self):
        # Arrange.
        self.fake()
        with patch.dict(
            os.environ,
            {
                "GITHUB_TOKEN": "unused",
                "OPENAI_API_KEY": "unused",
                "CODEX_HOME": "unused",
                "NODE_OPTIONS": "unused",
            },
        ):
            # Act.
            fields, reason = probe(self.binary)
        evidence = record(IDENTITY, fields, reason)
        calls = [json.loads(line) for line in self.trace.read_text().splitlines()]

        # Assert.
        self.validator.validate(evidence)
        self.assertEqual(evidence["outcome"], "verified")
        self.assertEqual(evidence["effective_model"], "gpt-6.1-sol")
        self.assertNotIn("RAW_PRIVATE", json.dumps(evidence))
        for call in calls:
            self.assertNotIn("GITHUB_TOKEN", call["env"])
            self.assertNotIn("OPENAI_API_KEY", call["env"])
            self.assertNotIn("CODEX_HOME", call["env"])
            self.assertNotIn("NODE_OPTIONS", call["env"])
            self.assertFalse(Path(call["cwd"]).exists())

        args = calls[-1]["args"]
        self.assertEqual(args[args.index("--model") + 1], "gpt-6.1-sol")
        self.assertIn('model_reasoning_effort="high"', args)
        for flag in ["--ignore-user-config", "--ignore-rules", "--ephemeral"]:
            self.assertIn(flag, args)
        self.assertEqual(args[args.index("--sandbox") + 1], "read-only")
        self.assertIn("features.shell_tool=false", args)
        self.assertIn("features.unified_exec=false", args)
        self.assertIn('web_search="disabled"', args)

    def test_safe_failure_records_cannot_claim_verified_policy(self):
        for failure, expected in {
            "version": "installation_unavailable",
            "auth": "authentication_unavailable",
            "response": "request_failed",
            "model": "effective_policy_unavailable",
            "effort": "effective_policy_unavailable",
            "ambiguous": "effective_policy_unavailable",
            "rerouted": "effective_policy_unavailable",
            "warning": "effective_policy_unavailable",
        }.items():
            with self.subTest(failure=failure):
                # Arrange.
                self.fake(failure)

                # Act.
                fields, reason = probe(self.binary)
                evidence = record(IDENTITY, fields, reason)

                # Assert.
                self.validator.validate(evidence)
                self.assertEqual(reason, expected)
                self.assertEqual(evidence["outcome"], "failed")
                self.assertIsNone(evidence["effective_model"])
                self.assertNotIn("RAW_PRIVATE", json.dumps(evidence))

    def test_missing_header_and_exceptions_fail_closed(self):
        def invoke(_binary, args, _directory, **_kwargs):
            import subprocess

            if "--version" in args:
                return subprocess.CompletedProcess(args, 0, b"codex-cli 0.157.1", b"")
            if "login" in args:
                return subprocess.CompletedProcess(
                    args, 0, b"Logged in using ChatGPT", b""
                )
            return subprocess.CompletedProcess(
                args, 0, b"HUB_CODEX_READY", b"RAW_PRIVATE"
            )

        fields, reason = probe(invoke=invoke)

        self.assertEqual(reason, "effective_policy_unavailable")
        self.validator.validate(record(IDENTITY, fields, reason))

        fields, reason = probe(
            invoke=lambda *_args, **_kwargs: (_ for _ in ()).throw(
                RuntimeError("RAW_PRIVATE")
            )
        )

        self.assertEqual(reason, "request_failed")
        self.assertNotIn("RAW_PRIVATE", json.dumps(record(IDENTITY, fields, reason)))

    def test_identity_is_validated_before_any_probe(self):
        with (
            patch.dict(os.environ, {"GITHUB_SHA": "untrusted"}),
            patch("profile_diagnostic.probe") as mocked,
        ):
            self.assertEqual(main(), 1)
            mocked.assert_not_called()

        for key in IDENTITY:
            identity = dict(IDENTITY)
            identity[key] = "RAW_PRIVATE"
            with self.assertRaises(ValueError):
                record(identity, {}, "request_failed")

        fields = {
            "cli_version": None,
            "effective_model": None,
            "effective_reasoning_effort": None,
        }
        now = datetime.datetime(2026, 10, 1, tzinfo=datetime.UTC)
        self.validator.validate(record(IDENTITY, fields, "request_failed", now))

    def test_workflow_is_private_manual_authorized_and_checksum_pinned(self):
        workflow_path = ROOT / ".github/workflows/agent-profile-diagnostic.yml"
        source = workflow_path.read_text()
        workflow = yaml.safe_load(source)

        # PyYAML uses YAML 1.1: the Actions 'on' key decodes as True.
        self.assertEqual(workflow[True], {"workflow_dispatch": None})
        self.assertEqual(workflow["permissions"], {})

        jobs = workflow["jobs"]
        for name in ["authorize", "diagnostic"]:
            self.assertIn("github.event.repository.private == true", jobs[name]["if"])
            self.assertIn("github.event.repository.fork == false", jobs[name]["if"])
            self.assertIn("github.event.repository.default_branch", jobs[name]["if"])

        diagnostic = jobs["diagnostic"]
        self.assertEqual(diagnostic["needs"], "authorize")
        self.assertEqual(diagnostic["permissions"], {})
        self.assertEqual(
            diagnostic["runs-on"], ["self-hosted", "linux", "hub-agent-codex"]
        )
        self.assertTrue(all("uses" not in step for step in diagnostic["steps"]))
        self.assertNotIn("actions/checkout", json.dumps(diagnostic))

        for filename in ["diagnose.py", "profile_diagnostic.py"]:
            checksum = hashlib.sha256(
                Path(__file__).with_name(filename).read_bytes()
            ).hexdigest()
            self.assertIn(checksum + "  " + filename, source)

        self.assertEqual(jobs["publish"]["runs-on"], "ubuntu-24.04")
        self.assertIn("agent-readiness-v1-${{ github.run_attempt }}", source)
        self.assertIn("TRIGGERING_ACTOR", source)
        self.assertNotIn("inputs.", source)
        self.assertNotIn("secrets.", source)


if __name__ == "__main__":
    unittest.main()
