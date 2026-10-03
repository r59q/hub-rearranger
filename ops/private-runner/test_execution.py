"""Offline AW-012 source/process boundary tests without authentication or a model."""

import hashlib
import io
import json
import os
import subprocess
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch as mock_patch

import jsonschema
import yaml

from execution import execute
from execution_config import (
    AUTH,
    CLI_VERSION,
    REQUIRED_FILES,
    ExecutionFailure,
    codex_arguments,
    environment,
    operator_config,
)
from execution_process import effective_policy, invoke
from execution_setup import manifest
from execution_workspace import baseline, extract, patch, protected

ROOT = Path(__file__).resolve().parents[2]


def invocation():
    return {
        "assignment_id": "42:99",
        "operation": "assignment",
        "profile_id": "codex-thorough",
        "profile_revision": "1" * 40,
        "policy_revision": "2" * 40,
        "base_sha": "2" * 40,
        "run_id": 100,
        "run_attempt": 1,
        "profile": {"context": {"sources": ["issue", "repository", "instructions"]}},
    }


def archive(files=None, special=None):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as file:
        for name, content in (
            files or {"app.txt": "before\n", "Makefile": "check:\n\ttrue\n"}
        ).items():
            entry = tarfile.TarInfo("checkout/" + name)
            entry.size = len(content.encode())
            entry.mode = 0o644
            file.addfile(entry, io.BytesIO(content.encode()))
        if special is not None:
            file.addfile(special)
    return output.getvalue()


class ExecutionTests(unittest.TestCase):
    def run_adapter(self, variant="ready"):
        directory = Path(self.temp.name)
        calls = []

        def fake(arguments, cwd, env, prompt=b"", timeout=120):
            calls.append((arguments, env, prompt))
            self.assertEqual(Path(arguments[0]).name, "codex")
            self.assertNotIn("GH_TOKEN", env)
            self.assertNotIn("OPENAI_API_KEY", env)
            if arguments[-1] == "--version":
                return subprocess.CompletedProcess(
                    arguments, 0, CLI_VERSION.encode(), b""
                )
            if arguments[-2:] == ["login", "status"]:
                return subprocess.CompletedProcess(
                    arguments, 0, b"", b"Logged in using ChatGPT"
                )
            if "sandbox" in arguments:
                if arguments[-1] == "check":
                    return subprocess.CompletedProcess(
                        arguments,
                        1 if variant == "validation" else 0,
                        b"private-sentinel",
                        b"",
                    )
                return subprocess.CompletedProcess(
                    arguments,
                    0,
                    b"ISOLATION_VERIFIED" if variant != "isolation" else b"",
                    b"",
                )
            if variant == "cancelled":
                raise ExecutionFailure("CANCELLED")
            if variant == "timeout":
                raise ExecutionFailure("RUNTIME_TIMEOUT")
            if variant != "empty":
                (cwd / "app.txt").write_text("after\n")
            if variant == "protected":
                (cwd / ".codex").mkdir()
                (cwd / ".codex/config.toml").write_text("private-sentinel")
            errors = b"model: gpt-6.1-sol\nreasoning effort: high\n"
            if variant == "model":
                errors = b"model: other\nreasoning effort: high\n"
            if variant == "reroute":
                errors += b"model rerouted: other\n"
            return subprocess.CompletedProcess(
                arguments, 0, b"private-sentinel", errors
            )

        with (
            mock_patch.dict(os.environ, {"GH_TOKEN": "private-sentinel"}),
            mock_patch("execution.RUNTIME", ROOT / "ops/private-runner"),
        ):
            result = execute(
                invocation(), archive(), {"issue": {"title": "Task"}}, directory, fake
            )
        schema = json.loads(
            (ROOT / "ops/private-runner/result.schema.v1.json").read_text()
        )
        jsonschema.Draft202012Validator(schema).validate(result)
        self.assertNotIn("private-sentinel", json.dumps(result))
        self.assertFalse(list(directory.glob("hub-agent-work-*")))
        return result, calls

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)

    def test_patch_is_independent_of_model_authored_metadata(self):
        result, calls = self.run_adapter()
        self.assertEqual(result["outcome"], "ready")
        self.assertEqual(result["validation"][0]["outcome"], "passed")
        output = Path(self.temp.name) / "output"
        self.assertIn(b"+after", (output / "proposal.patch").read_bytes())
        self.assertNotIn(b"private-sentinel", (output / "summary.json").read_bytes())
        execution = next(args for args, _, _ in calls if "exec" in args)
        self.assertIn("--ignore-user-config", execution)
        self.assertIn("--ignore-rules", execution)
        self.assertIn("--ephemeral", execution)
        self.assertNotIn("--dangerously-bypass-approvals-and-sandbox", execution)
        self.assertEqual(execution[execution.index("--model") + 1], "gpt-6.1-sol")

    def test_validation_failure_keeps_proposal_and_explicit_evidence(self):
        result, _ = self.run_adapter("validation")
        self.assertEqual(result["outcome"], "ready")
        self.assertEqual(result["validation"][0]["outcome"], "failed")

    def test_failures_never_publish_a_partial_patch(self):
        for variant, reason in {
            "model": "MODEL_POLICY_UNSUPPORTED",
            "reroute": "MODEL_POLICY_UNSUPPORTED",
            "empty": "NO_CHANGES",
            "protected": "PROTECTED_CHANGE",
            "cancelled": "CANCELLED",
            "timeout": "RUNTIME_TIMEOUT",
            "isolation": "RUNNER_NOT_READY",
        }.items():
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as name:
                previous = self.temp
                self.temp = type("Directory", (), {"name": name})()
                try:
                    result, _ = self.run_adapter(variant)
                finally:
                    self.temp = previous
                self.assertEqual(result["reason_code"], reason)
                self.assertIsNone(result["proposal"])
                self.assertEqual(
                    [p.name for p in (Path(name) / "output").iterdir()], ["result.json"]
                )

    def test_model_headers_must_be_unique_and_have_no_warning(self):
        for errors in (
            b"",
            b"model: gpt-6.1-sol\n",
            b"model: gpt-6.1-sol\nmodel: gpt-6.1-sol\nreasoning effort: high\n",
            b"model: gpt-6.1-sol\nreasoning effort: high\nWARNING: changed\n",
        ):
            with self.assertRaises(ExecutionFailure):
                effective_policy(subprocess.CompletedProcess([], 0, b"", errors))

    def test_process_has_bounded_output_and_deadline(self):
        directory = Path(self.temp.name)
        with self.assertRaisesRegex(ExecutionFailure, "RUNTIME_TIMEOUT"):
            invoke(
                [sys.executable, "-c", "import time; time.sleep(5)"],
                directory,
                {},
                timeout=0.05,
            )
        with mock_patch("execution_process.MAX_OUTPUT", 1000):
            with self.assertRaisesRegex(ExecutionFailure, "OUTPUT_LIMIT_EXCEEDED"):
                invoke([sys.executable, "-c", "print('x'*2000)"], directory, {})

    def test_workload_environment_is_explicit_and_does_not_expose_auth_home(self):
        directory = Path(self.temp.name)
        scratch = directory / "scratch"
        scratch.mkdir()
        env = environment(directory, scratch)
        self.assertEqual(set(env), {"HOME", "TMPDIR", "PATH", "LANG", "CODEX_HOME"})
        self.assertNotEqual(env["CODEX_HOME"], str(AUTH))
        args = codex_arguments(directory / "source", scratch)
        self.assertIn("permissions.aw012.network.enabled=false", args)
        self.assertIn("features.multi_agent=false", args)
        self.assertIn("features.multi_agent_v2=false", args)
        self.assertIn('shell_environment_policy.inherit="none"', args)
        policy = next(
            value for value in args if value.startswith("permissions.aw012.filesystem=")
        )
        self.assertIn('":root"="deny"', policy)
        self.assertIn(json.dumps(str(AUTH)) + '="deny"', policy)


class WorkspaceTests(unittest.TestCase):
    def test_repository_config_and_credentials_are_not_materialized(self):
        with tempfile.TemporaryDirectory() as name:
            workspace = Path(name)
            rules = extract(
                archive(
                    {
                        "app.txt": "source",
                        "AGENTS.md": "repository instruction",
                        ".codex/config.toml": "invalid config",
                        ".env": "synthetic-secret",
                    }
                ),
                workspace,
                instructions=True,
            )
            self.assertEqual(
                rules, [{"path": "AGENTS.md", "body": "repository instruction"}]
            )
            self.assertEqual([p.name for p in workspace.iterdir()], ["app.txt"])
            self.assertEqual(
                extract(archive({"AGENTS.md": "rule"}), workspace, False), []
            )

    def test_tar_paths_links_and_submodules_are_rejected(self):
        for name, kind in (
            ("checkout/../escape", tarfile.REGTYPE),
            ("/absolute", tarfile.REGTYPE),
            ("checkout/link", tarfile.SYMTYPE),
            ("checkout/link", tarfile.LNKTYPE),
            ("checkout/device", tarfile.CHRTYPE),
            ("checkout/.gitmodules", tarfile.REGTYPE),
        ):
            with self.subTest(path=name), tempfile.TemporaryDirectory() as directory:
                entry = tarfile.TarInfo(name)
                entry.type = kind
                with self.assertRaises(ExecutionFailure):
                    extract(archive(special=entry), Path(directory))

    def test_capture_includes_new_source_but_excludes_ignored_build_output(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            workspace = root / "source"
            workspace.mkdir()
            extract(archive({"app.txt": "old", ".gitignore": "build/\n"}), workspace)
            tree = baseline(root / "git", workspace)
            (workspace / "new.txt").write_text("new source")
            (workspace / "build").mkdir()
            (workspace / "build/generated.txt").write_text("generated")
            content = patch(root / "git", workspace, tree)
            self.assertIn(b"new.txt", content)
            self.assertNotIn(b"generated.txt", content)

    def test_existing_protected_changes_and_new_unsafe_links_are_rejected(self):
        for variant in ("workflow", "symlink", "instructions"):
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as name:
                root = Path(name)
                workspace = root / "source"
                workspace.mkdir()
                extract(
                    archive({"app.txt": "source", ".github/workflows/a.yml": "old"}),
                    workspace,
                )
                tree = baseline(root / "git", workspace)
                if variant == "workflow":
                    (workspace / ".github/workflows/a.yml").write_text("changed")
                elif variant == "symlink":
                    (workspace / "link").symlink_to("/etc/passwd")
                else:
                    (workspace / "AGENTS.md").write_text("new policy")
                with self.assertRaisesRegex(ExecutionFailure, "PROTECTED_CHANGE"):
                    patch(root / "git", workspace, tree)

    def test_intake_helpers_are_protected(self):
        self.assertTrue(protected("ops/agent-intake/cmd/patch/main.go"))


class OperatorTests(unittest.TestCase):
    def test_disabled_installation_and_modified_helper_cannot_execute(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            runtime, work, auth = root / "runtime", root / "state", root / "auth"
            work.mkdir(mode=0o700)
            auth.mkdir()
            (auth / "auth.json").write_text("synthetic-auth")
            (auth / "auth.json").chmod(0o600)
            for relative in REQUIRED_FILES:
                path = runtime / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("reviewed-file")
            config = {
                "GITHUB_REPOSITORY": "r59q/hub-rearranger",
                "RUNNER_NAME": "addons",
            }
            with (
                mock_patch("execution_setup.RUNTIME", runtime),
                mock_patch("execution_config.RUNTIME", runtime),
                mock_patch("execution_config.WORK", work),
                mock_patch("execution_config.AUTH", auth),
                mock_patch("execution_config.MANIFEST", runtime / "operator.json"),
                mock_patch("execution_config.trusted", side_effect=lambda path: path),
            ):
                record = manifest("r59q/hub-rearranger", "addons")
                (runtime / "operator.json").write_text(json.dumps(record))
                operator_config(config, require_enabled=False)
                with self.assertRaises(ExecutionFailure):
                    operator_config(config)
                record["enabled"] = True
                (runtime / "operator.json").write_text(json.dumps(record))
                operator_config(config)
                with self.assertRaises(ExecutionFailure):
                    operator_config({**config, "RUNNER_NAME": "different-runner"})
                (runtime / "codex-code-mode-host").write_text("unreviewed-change")
                with self.assertRaises(ExecutionFailure):
                    operator_config(config)

    def test_shared_runner_exception_cannot_be_reused_for_another_repository(self):
        with self.assertRaises(ValueError):
            manifest("r59q/another-repository", "addons")


class WorkflowTests(unittest.TestCase):
    def test_runner_gate_and_read_only_trust_split(self):
        workflow = yaml.safe_load(
            (ROOT / ".github/workflows/agent-assignment.yml").read_text()
        )
        jobs = workflow["jobs"]
        job = jobs["patch"]
        self.assertEqual(job["runs-on"], ["self-hosted", "linux", "hub-agent-codex"])
        self.assertEqual(
            job["permissions"],
            {"contents": "read", "actions": "read", "issues": "read"},
        )
        prefix = (
            "needs.authorize.outputs.dispatch == 'true' && "
            "needs.dispatch.outputs.execute == 'true' && "
        )
        suffix = (
            " && github.event.repository.fork == false && "
            "vars.HUB_AGENT_EXECUTION_ENABLED == 'verified' && "
            "github.ref == format('refs/heads/{0}', "
            "github.event.repository.default_branch)"
        )
        private = "github.event.repository.private == true"
        # Portable bootstrap narrows the exact upstream operator exception.
        # Both forms retain every authority/default-branch/no-fork/enablement gate.
        self.assertIn(
            job["if"],
            {
                prefix + private + suffix,
                prefix
                + f"({private} || github.repository == 'r59q/hub-rearranger')"
                + suffix,
            },
        )
        for step in job["steps"]:
            if "uses" in step:
                self.assertEqual(len(step["uses"].split("@")[1]), 40)
                self.assertNotIn("checkout", step["uses"])
            if "run" in step:
                self.assertNotIn("${{", step["run"])
        self.assertEqual(jobs["report"]["permissions"], {})
        self.assertIn("always()", jobs["report"]["if"])
        checksum = hashlib.sha256(
            (ROOT / "ops/private-runner/execution.py").read_bytes()
        ).hexdigest()
        self.assertIn(checksum + "  execution.py", job["steps"][0]["run"])


if __name__ == "__main__":
    unittest.main()
