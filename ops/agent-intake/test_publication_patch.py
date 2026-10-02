"""Real Git/index publication boundary tests; source recipes never execute."""

import base64
import io
import os
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch as mock_patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "private-runner"))

from execution_config import ExecutionFailure
from execution_workspace import baseline, patch
from publication_patch import apply


def archive(files):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as tar:
        for name, content in files.items():
            info = tarfile.TarInfo("synthetic/" + name)
            info.size, info.mode = len(content), 0o644
            tar.addfile(info, io.BytesIO(content))
    return output.getvalue()


def addition(path, mode="100644", content="new"):
    return (
        f"diff --git a/{path} b/{path}\nnew file mode {mode}\n"
        f"--- /dev/null\n+++ b/{path}\n@@ -0,0 +1 @@\n+{content}\n"
    ).encode()


class PublicationPatchTests(unittest.TestCase):
    def test_index_patch_creates_changes_without_running_workload(self):
        with tempfile.TemporaryDirectory() as name:
            marker = Path(name) / "canary"
            source = archive(
                {
                    "app.txt": b"before\n",
                    "Makefile": f"check:\n\ttouch {marker}\n".encode(),
                    ".gitattributes": b"*.txt filter=canary\n",
                }
            )
            delta = (
                b"diff --git a/app.txt b/app.txt\n--- a/app.txt\n+++ b/app.txt\n"
                b"@@ -1 +1 @@\n-before\n+after\n"
            )
            injected = {
                "GH_TOKEN": "synthetic-sentinel",
                "GIT_CONFIG_COUNT": "1",
                "GIT_CONFIG_KEY_0": "filter.canary.clean",
                "GIT_CONFIG_VALUE_0": f"touch {marker}",
            }
            with mock_patch.dict(os.environ, injected):
                changes = apply(source, delta)
            self.assertEqual(
                changes,
                [
                    {
                        "path": "app.txt",
                        "mode": "100644",
                        "content": base64.b64encode(b"after\n").decode(),
                        "delete": False,
                    }
                ],
            )
            self.assertFalse(marker.exists())

    def test_protected_and_escaping_paths_are_rejected(self):
        for path in (
            ".github/workflows/write.yml",
            "ops/private-runner/code.py",
            "ops/agent-profiles/code.py",
            "ops/agent-intake/code.py",
            "AGENTS.md",
            "docs/AGENTS.md",
            ".codex/config.toml",
            ".agents/config",
            ".git/config",
            ".env",
            "docs/.env.secret",
            ".gitmodules",
            "../escape",
            "/tmp/escape",
            "docs/../escape",
            "docs\\escape",
        ):
            with self.subTest(path=path), self.assertRaises(ExecutionFailure):
                apply(archive({"app.txt": b"before\n"}), addition(path))

    def test_unsafe_modes_and_stale_patch_are_rejected(self):
        for delta in (
            addition("link", "120000", "../../host"),
            addition("module", "160000", "Subproject commit " + "1" * 40),
            (
                b"diff --git a/app.txt b/app.txt\n--- a/app.txt\n+++ b/app.txt\n"
                b"@@ -1 +1 @@\n-stale\n+after\n"
            ),
            b"not a patch",
        ):
            with self.subTest(delta=delta[:40]), self.assertRaises(ExecutionFailure):
                apply(archive({"app.txt": b"before\n"}), delta)

    def test_binary_delete_and_executable_changes_use_real_git(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            workspace = root / "source"
            workspace.mkdir()
            before = {
                "binary.dat": b"\x00before\xff",
                "remove.txt": b"remove\n",
                "script.sh": b"echo safe\n",
            }
            for path, content in before.items():
                (workspace / path).write_bytes(content)
            tree = baseline(root / "git", workspace)
            (workspace / "binary.dat").write_bytes(b"\x00after\xfe")
            (workspace / "remove.txt").unlink()
            (workspace / "script.sh").chmod(0o755)
            delta = patch(root / "git", workspace, tree)
            changes = {
                change["path"]: change for change in apply(archive(before), delta)
            }
            self.assertEqual(
                base64.b64decode(changes["binary.dat"]["content"]), b"\x00after\xfe"
            )
            self.assertTrue(changes["remove.txt"]["delete"])
            self.assertEqual(changes["script.sh"]["mode"], "100755")
