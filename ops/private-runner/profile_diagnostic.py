#!/usr/bin/env python3
"""Fixed AW-006 no-write probe; install alongside the checksum-pinned AW-001 probe."""

import datetime
import json
import os
import re
import runpy
import tempfile
from pathlib import Path

MODEL = "gpt-6.1-sol"
EFFORT = "high"
PROFILE = "codex-thorough"
RUNNER = "hub-agent-codex"


def probe(binary=None, invoke=None):
    if invoke is None:
        invoke = runpy.run_path(str(Path(__file__).with_name("diagnose.py")))["invoke"]
    if binary is None:
        binary = Path.home() / ".local/lib/hub-agent-runtime/codex"

    fields = {
        "cli_version": None,
        "effective_model": None,
        "effective_reasoning_effort": None,
    }

    with tempfile.TemporaryDirectory(prefix="codex-profile-diagnostic-") as directory:
        try:
            version = invoke(binary, ["--version"], directory)
            if (
                version.returncode
                or not re.fullmatch(
                    rb"codex-cli [0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.-]+)?\s*",
                    version.stdout,
                )
                or len(version.stdout) > 100
            ):
                return fields, "installation_unavailable"
            fields["cli_version"] = version.stdout.decode("ascii").strip()

            auth = [
                "-c",
                'forced_login_method="chatgpt"',
                "-c",
                'cli_auth_credentials_store="file"',
            ]
            status = invoke(binary, [*auth, "login", "status"], directory)
            if status.returncode or (status.stdout + status.stderr).strip() != (
                b"Logged in using ChatGPT"
            ):
                return fields, "authentication_unavailable"

            result = invoke(
                binary,
                [
                    *auth,
                    "exec",
                    "--ignore-user-config",
                    "--ignore-rules",
                    "--ephemeral",
                    "--skip-git-repo-check",
                    "--sandbox",
                    "read-only",
                    "--color",
                    "never",
                    "--model",
                    MODEL,
                    "-c",
                    f'model_reasoning_effort="{EFFORT}"',
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
                    "Reply with exactly HUB_CODEX_READY. "
                    "Do not use tools or read files.",
                ],
                directory,
                timeout=120,
            )
            if result.returncode or result.stdout.strip() != b"HUB_CODEX_READY":
                return fields, "request_failed"

            # Verify effective session policy from the CLI's startup header,
            # never from the requested arguments or model-authored answer. A
            # changed/missing/ambiguous header fails closed. Do not log stderr.
            models = re.findall(rb"^model: +([^\r\n]+)$", result.stderr, re.MULTILINE)
            efforts = re.findall(
                rb"^reasoning effort: +([^\r\n]+)$", result.stderr, re.MULTILINE
            )
            if (
                models != [MODEL.encode()]
                or efforts != [EFFORT.encode()]
                or re.search(
                    rb"(?im)^(?:warning\b|model (?:rerouted|changed)\b)", result.stderr
                )
            ):
                return fields, "effective_policy_unavailable"

            fields["effective_model"] = MODEL
            fields["effective_reasoning_effort"] = EFFORT
            return fields, "verified"
        except Exception:
            # Includes sanitized legacy ProbeFailure; never stringify exceptions.
            return fields, "request_failed"


def record(environment, fields, reason, now=None):
    repository = environment.get("GITHUB_REPOSITORY", "")
    revision = environment.get("GITHUB_SHA", "")
    run_id = environment.get("GITHUB_RUN_ID", "")
    attempt = environment.get("GITHUB_RUN_ATTEMPT", "")
    if (
        not re.fullmatch(
            r"[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9_.-]{1,100}", repository
        )
        or not re.fullmatch(r"[0-9a-f]{40}", revision)
        or not re.fullmatch(r"[1-9][0-9]{0,15}", run_id)
        or int(run_id) > 9007199254740991
        or not re.fullmatch(r"[1-9][0-9]{0,4}", attempt)
        or int(attempt) > 10000
    ):
        raise ValueError("Invalid diagnostic identity")

    return {
        "version": 1,
        "repository": repository,
        "profile_id": PROFILE,
        "profile_revision": revision,
        "runner_label": RUNNER,
        "requested_model": MODEL,
        "requested_reasoning_effort": EFFORT,
        **fields,
        "run_id": int(run_id),
        "run_attempt": int(attempt),
        "verified_at": (now or datetime.datetime.now(datetime.UTC)).isoformat(),
        "outcome": "verified" if reason == "verified" else "failed",
        "reason_code": reason,
    }


def main():
    # Validate identity before accessing any authentication or live provider.
    try:
        record(os.environ, {}, "request_failed")
    except ValueError:
        print("Diagnostic metadata unavailable.")
        return 1

    fields, reason = probe()
    print(json.dumps(record(os.environ, fields, reason), separators=(",", ":")))

    # A matching failed probe is evidence too. Workflow infrastructure failures
    # produce no artifact and remain pending rather than manufacturing a failure.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
