"""Exercise the process boundary without using real credentials or a model."""

import hashlib
import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

import yaml

from diagnose import ProbeFailure, diagnose, invoke


class DiagnosticTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.binary = self.root / "codex"
        self.trace = self.root / "calls"

    def fake_codex(self, failure=""):
        self.binary.write_text(
            "#!/usr/bin/python3\n"
            "import json, os, sys\n"
            f"failure = {failure!r}\n"
            f"with open({str(self.trace)!r}, 'a') as trace:\n"
            "    trace.write(json.dumps({'args': sys.argv[1:], "
            "'env': dict(os.environ), "
            "'cwd': os.getcwd()}) + '\\n')\n"
            "if '--version' in sys.argv:\n"
            "    print('RAW_OUTPUT' if failure == 'version' else 'codex-cli 0.157.1')\n"
            "elif 'login' in sys.argv:\n"
            "    print('RAW_OUTPUT' if failure == 'auth' "
            "else 'Logged in using ChatGPT', "
            "file=sys.stderr)\n"
            "    sys.exit(1 if failure == 'signed-out' else 0)\n"
            "else:\n"
            "    print('RAW_OUTPUT', file=sys.stderr)\n"
            "    print('RAW_OUTPUT' if failure == 'response' else 'HUB_CODEX_READY')\n"
            "    sys.exit(1 if failure == 'exec' else 0)\n"
        )
        self.binary.chmod(0o700)

    def test_success_uses_isolated_context_and_publishes_only_safe_results(self):
        self.fake_codex()
        output = io.StringIO()
        injected = {
            name: "unused"
            for name in [
                "GITHUB_TOKEN",
                "OPENAI_API_KEY",
                "CODEX_API_KEY",
                "CODEX_ACCESS_TOKEN",
                "CODEX_HOME",
                "NODE_OPTIONS",
                "OPENAI_BASE_URL",
            ]
        }
        with redirect_stdout(output), patch.dict(os.environ, injected):
            diagnose(self.binary)

        calls = [json.loads(line) for line in self.trace.read_text().splitlines()]
        self.assertEqual(len(calls), 3)
        self.assertIn("Live subscription request: verified", output.getvalue())
        self.assertNotIn("RAW_OUTPUT", output.getvalue())
        for call in calls:
            self.assertTrue(injected.keys().isdisjoint(call["env"]))
            self.assertFalse(Path(call["cwd"]).exists())
        self.assertIn("--ephemeral", calls[-1]["args"])
        self.assertIn("read-only", calls[-1]["args"])
        self.assertIn('forced_login_method="chatgpt"', calls[-1]["args"])
        self.assertIn("--ignore-user-config", calls[-1]["args"])
        self.assertIn("--ignore-rules", calls[-1]["args"])
        self.assertIn('approval_policy="never"', calls[-1]["args"])

    def test_authentication_failure_stops_before_live_request(self):
        self.fake_codex("auth")

        with redirect_stdout(io.StringIO()), self.assertRaises(ProbeFailure):
            diagnose(self.binary)

        calls = [json.loads(line) for line in self.trace.read_text().splitlines()]
        self.assertEqual(len(calls), 2)

    def test_failures_do_not_publish_raw_output_or_report_success(self):
        for failure in ["version", "auth", "signed-out", "exec", "response"]:
            with self.subTest(failure=failure):
                self.fake_codex(failure)
                output = io.StringIO()
                with redirect_stdout(output), self.assertRaises(ProbeFailure) as raised:
                    diagnose(self.binary)

                self.assertNotIn(
                    "RAW_OUTPUT", str(raised.exception) + output.getvalue()
                )
                self.assertNotIn("verified", output.getvalue())

    def test_missing_binary_has_safe_recovery_message(self):
        with self.assertRaisesRegex(ProbeFailure, "installation"):
            diagnose(self.binary)

    def test_timeout_has_safe_recovery_message(self):
        self.binary.write_text("#!/usr/bin/python3\nimport time\ntime.sleep(10)\n")
        self.binary.chmod(0o700)

        with self.assertRaisesRegex(ProbeFailure, "timed out"):
            invoke(self.binary, [], self.root, timeout=0.1)

    def test_workflow_pins_the_reviewed_probe(self):
        repository = Path(__file__).resolve().parents[2]
        digest = hashlib.sha256(
            (repository / "ops/private-runner/diagnose.py").read_bytes()
        )
        workflow = (
            repository / ".github/workflows/agent-runtime-diagnostic.yml"
        ).read_text()

        self.assertIn(f"{digest.hexdigest()}  diagnose.py", workflow)

    def test_workflow_keeps_credentials_away_from_untrusted_events_and_checkout(self):
        repository = Path(__file__).resolve().parents[2]
        workflow = yaml.load(
            (repository / ".github/workflows/agent-runtime-diagnostic.yml").read_text(),
            Loader=yaml.BaseLoader,
        )

        self.assertEqual(set(workflow["on"]), {"workflow_dispatch", "push"})
        self.assertEqual(workflow["permissions"], {})
        authorization = workflow["jobs"]["authorize"]
        self.assertEqual(authorization["runs-on"], "ubuntu-24.04")
        self.assertEqual(authorization["permissions"], {"contents": "read"})
        self.assertIn("github.event.repository.fork == false", authorization["if"])
        job = workflow["jobs"]["diagnostic"]
        self.assertEqual(job["needs"], "authorize")
        for restriction in [
            "needs.authorize.result == 'success'",
            "github.event.repository.fork == false",
            "github.ref == format('refs/heads/{0}', "
            "github.event.repository.default_branch)",
        ]:
            self.assertIn(restriction, job["if"])
        self.assertEqual(job["runs-on"], ["self-hosted", "linux"])
        self.assertTrue(all("uses" not in step for step in job["steps"]))


if __name__ == "__main__":
    unittest.main()
