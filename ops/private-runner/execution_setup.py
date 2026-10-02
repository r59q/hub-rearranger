#!/usr/bin/env python3
"""Create a disabled operator manifest or run fixed, source-free preflight."""

import argparse
import datetime
import hashlib
import json
import re
import runpy
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from execution import isolation
from execution_config import (
    CLI_VERSION,
    REQUIRED_FILES,
    RUNTIME,
    WORK,
    codex_arguments,
    environment,
    operator_config,
)
from execution_process import invoke


def manifest(repository, runner):
    if not re.fullmatch(
        r"[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9_.-]{1,100}", repository
    ):
        raise ValueError
    if not re.fullmatch(r"[A-Za-z0-9_-]{1,64}", runner):
        raise ValueError
    if runner == "addons" and repository != "r59q/hub-rearranger":
        raise ValueError
    return {
        "version": 1,
        "enabled": False,
        "repository": repository,
        "runner_name": runner,
        "cli_version": CLI_VERSION,
        "sha256": {
            name: hashlib.sha256((RUNTIME / name).read_bytes()).hexdigest()
            for name in sorted(REQUIRED_FILES)
        },
    }


def preflight(repository, runner):
    operator_config(
        {"GITHUB_REPOSITORY": repository, "RUNNER_NAME": runner}, require_enabled=False
    )
    with tempfile.TemporaryDirectory(prefix="hub-agent-preflight-", dir=WORK) as name:
        root = Path(name)
        workspace, scratch = root / "source", root / "scratch"
        workspace.mkdir()
        scratch.mkdir()
        isolation(RUNTIME / "codex", workspace, scratch)
        probe = runpy.run_path(str(RUNTIME / "profile_diagnostic.py"))["probe"]
        env = environment(workspace, scratch, auth=True)

        def invoke_probe(binary, arguments, directory, timeout=120):
            prompt = b""
            if "exec" in arguments:
                arguments = codex_arguments(workspace, scratch)
                prompt = (
                    b"Reply with exactly HUB_CODEX_READY. "
                    b"Do not use tools or read files."
                )
            return invoke([binary, *arguments], workspace, env, prompt, timeout)

        fields, reason = probe(binary=RUNTIME / "codex", invoke=invoke_probe)
    return {
        "version": 1,
        "repository": repository,
        "runner_name": runner,
        "isolation": "verified",
        **fields,
        "reason_code": reason,
        "verified_at": datetime.datetime.now(datetime.UTC).isoformat(),
        "github_evidence": "pending",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("repository")
    parser.add_argument("runner")
    parser.add_argument("--preflight", action="store_true")
    args = parser.parse_args()
    try:
        if args.preflight:
            record = preflight(args.repository, args.runner)
            print(json.dumps(record, sort_keys=True))
            return 0 if record["reason_code"] == "verified" else 1
        print(
            json.dumps(manifest(args.repository, args.runner), indent=2, sort_keys=True)
        )
        return 0
    except Exception:
        print('{"reason_code":"RUNNER_NOT_READY"}')
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
