#!/usr/bin/env python3
"""Apply a proposal to a private index; never execute or check out patched code."""

import argparse
import base64
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "private-runner"))

from execution_config import ExecutionFailure, fail
from execution_workspace import MAX_PATCH, MAX_SOURCE, baseline, extract, protected


def git(directory, arguments, payload=None):
    response = subprocess.run(
        [
            "/usr/bin/git",
            "--no-optional-locks",
            "-c",
            "core.hooksPath=/dev/null",
            "-c",
            "core.attributesFile=/dev/null",
            "--git-dir=" + str(directory),
            *arguments,
        ],
        env={
            "PATH": "/usr/bin:/bin",
            "LANG": "C.UTF-8",
            "HOME": str(directory.parent),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_SYSTEM": "/dev/null",
            "GIT_ATTR_NOSYSTEM": "1",
        },
        input=payload,
        capture_output=True,
        timeout=60,
        cwd=directory.parent,
    )
    if response.returncode:
        fail("PROTECTED_CHANGE")
    return response.stdout


def valid_path(path):
    parts = path.split("/")
    return (
        bool(path)
        and len(path.encode()) <= 4096
        and all(p not in {"", ".", ".."} for p in parts)
        and "\\" not in path
        and not any(ord(c) < 32 or ord(c) == 127 for c in path)
        and not protected(path)
    )


def apply(source, patch):
    if not 0 < len(patch) <= MAX_PATCH:
        fail("PATCH_LIMIT_EXCEEDED")
    with tempfile.TemporaryDirectory(prefix="agent-publish-index-") as name:
        private = Path(name)
        private.chmod(0o700)
        workspace, directory = private / "source", private / "git"
        workspace.mkdir(mode=0o700)
        extract(source, workspace)
        tree = baseline(directory, workspace)
        # Cached application never materializes malicious links/files in the
        # workspace or loads repository commands, hooks, filters, or config.
        git(directory, ["apply", "--cached", "--check", "-"], patch)
        git(directory, ["apply", "--cached", "-"], patch)
        names = git(
            directory, ["diff", "--cached", "--no-renames", "--name-only", "-z", tree]
        ).split(b"\0")
        paths = [p.decode("utf-8") for p in names if p]
        if not paths or len(paths) > 1000 or any(not valid_path(p) for p in paths):
            fail("PROTECTED_CHANGE")
        entries = {}
        for entry in git(directory, ["ls-files", "--stage", "-z"]).split(b"\0"):
            if not entry:
                continue
            metadata, path = entry.split(b"\t", 1)
            mode, oid, stage = metadata.split(b" ")
            if mode not in {b"100644", b"100755"} or stage != b"0":
                fail("PROTECTED_CHANGE")
            entries[path.decode("utf-8")] = (mode.decode(), oid.decode())
        changes, total = [], 0
        for path in paths:
            mode, oid = entries.get(path, ("100644", None))
            content = git(directory, ["cat-file", "blob", oid]) if oid else b""
            total += len(content)
            if len(content) > 16 * 1024 * 1024 or total > MAX_SOURCE:
                fail("PATCH_LIMIT_EXCEEDED")
            changes.append(
                {
                    "path": path,
                    "mode": mode,
                    "content": base64.b64encode(content).decode(),
                    "delete": oid is None,
                }
            )
        return changes


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True)
    parser.add_argument("--patch", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        source = Path(args.source).read_bytes()
        patch = Path(args.patch).read_bytes()
        if len(source) > 32 * 1024 * 1024:
            fail("SOURCE_LIMIT_EXCEEDED")
        changes = apply(source, patch)
        descriptor = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w") as stream:
            json.dump(changes, stream)
    except ExecutionFailure as error:
        print(json.dumps({"code": str(error)}))
        return 1
    except Exception:
        print('{"code":"PROTECTED_CHANGE"}')
        return 1
    print('{"code":"VERIFIED"}')
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
