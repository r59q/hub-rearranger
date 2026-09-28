#!/usr/bin/env python3
"""AW-001: fixed, credential-safe probe installed by the runner administrator."""

import os
import re
import subprocess
import tempfile
from pathlib import Path


class ProbeFailure(Exception):
    """A message safe to publish in a GitHub Actions log."""


def invoke(binary, arguments, directory, timeout=30):
    # Inherit the service user's actual home, but no GitHub/API tokens, provider
    # overrides, injected Node options, or an unrelated CODEX_HOME.
    environment = {
        "HOME": os.environ["HOME"],
        "PATH": "/usr/local/bin:/usr/bin:/bin",
        "LANG": "C.UTF-8",
    }
    try:
        return subprocess.run(
            [str(binary), *arguments],
            cwd=directory,
            env=environment,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        raise ProbeFailure(
            "Codex timed out; inspect runner connectivity locally."
        ) from None
    except OSError:
        raise ProbeFailure(
            "Codex could not start; inspect its installation locally."
        ) from None


def diagnose(binary=None):
    if binary is None:
        binary = Path.home() / ".local/lib/hub-agent-runtime/codex"
    # No checkout, repository instructions, input text, or previous session.
    with tempfile.TemporaryDirectory(prefix="codex-diagnostic-") as directory:
        version = invoke(binary, ["--version"], directory)
        if version.returncode or not re.fullmatch(
            rb"codex-cli [0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.-]+)?\s*",
            version.stdout,
        ):
            raise ProbeFailure(
                "Codex version unavailable; inspect the pinned installation."
            )
        print(version.stdout.decode("ascii").strip(), flush=True)

        auth_options = [
            "-c",
            'forced_login_method="chatgpt"',
            "-c",
            'cli_auth_credentials_store="file"',
        ]
        status = invoke(binary, [*auth_options, "login", "status"], directory)
        # Fail closed when the CLI's status format changes. Never forward its
        # output: other login methods can include credential fragments.
        if status.returncode or (status.stdout + status.stderr).strip() != (
            b"Logged in using ChatGPT"
        ):
            raise ProbeFailure(
                "ChatGPT login not ready; sign in on the runner and retry."
            )
        print("Cached ChatGPT login: present", flush=True)

        result = invoke(
            binary,
            [
                *auth_options,
                "exec",
                "--ignore-user-config",
                "--ignore-rules",
                "--ephemeral",
                "--skip-git-repo-check",
                "--sandbox",
                "read-only",
                "--color",
                "never",
                "-c",
                'approval_policy="never"',
                "-c",
                "features.shell_tool=false",
                "-c",
                "features.unified_exec=false",
                "-c",
                "features.shell_snapshot=false",
                "-c",
                'web_search="disabled"',
                "Reply with exactly HUB_CODEX_READY. Do not use tools or read files.",
            ],
            directory,
            timeout=120,
        )
        if result.returncode or result.stdout.strip() != b"HUB_CODEX_READY":
            raise ProbeFailure(
                "Live verification failed; inspect login, quota, CLI compatibility, "
                "and connectivity locally."
            )
        print("Live subscription request: verified", flush=True)


def main():
    try:
        diagnose()
    except (ProbeFailure, OSError, KeyError) as error:
        message = (
            str(error)
            if isinstance(error, ProbeFailure)
            else "Runner setup is incomplete."
        )
        print(message, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
