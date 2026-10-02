#!/usr/bin/env python3
"""Canonical evidence/result checks for the trusted read-only execution gate."""

import datetime
import hashlib
import json
import sys
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parents[1] / "private-runner"


def verify(request, now=None):
    input = request["invocation"]
    record = request["readiness"]
    schema = json.loads((ROOT / "readiness.schema.v1.json").read_text())
    Draft202012Validator(schema, format_checker=FormatChecker()).validate(record)
    timestamp = datetime.datetime.fromisoformat(record["verified_at"])
    now = now or datetime.datetime.now(datetime.UTC)
    if not now - datetime.timedelta(hours=24) <= timestamp <= now:
        raise ValueError
    expected = {
        "repository": input["repository"]["owner"] + "/" + input["repository"]["name"],
        "profile_id": input["profile_id"],
        "profile_revision": input["policy_revision"],
        "runner_label": "hub-agent-codex",
        "requested_model": "gpt-6.1-sol",
        "effective_model": "gpt-6.1-sol",
        "requested_reasoning_effort": "high",
        "effective_reasoning_effort": "high",
        "outcome": "verified",
        "reason_code": "verified",
        "cli_version": "codex-cli 0.159.3",
    }
    if any(record[key] != value for key, value in expected.items()):
        raise ValueError
    profile = input["profile"]
    if profile["model"] != {
        "id": "gpt-6.1-sol",
        "reasoning_effort": "high",
        "fallback": "none",
    } or profile["authority"] != {
        "mode": "branch-draft-pr",
        "sandbox": "workspace-write",
        "network": False,
    }:
        raise ValueError

    previous = request.get("previous")
    if previous is not None:
        result = previous["result"]
        schema = json.loads((ROOT / "result.schema.v1.json").read_text())
        Draft202012Validator(schema).validate(result)
        keys = (
            "assignment_id",
            "operation",
            "profile_id",
            "profile_revision",
            "base_sha",
            "run_id",
        )
        if any(result[key] != input[key] for key in keys):
            raise ValueError
        if result["outcome"] != "ready" or not (
            0 < result["run_attempt"] < input["run_attempt"]
        ):
            raise ValueError
        if result["head_sha"] != input["base_sha"]:
            raise ValueError
        if (
            result["proposal"]["patch_sha256"]
            != hashlib.sha256(bytes.fromhex(previous["patch_hex"])).hexdigest()
        ):
            raise ValueError
        if (
            result["proposal"]["summary_sha256"]
            != hashlib.sha256(bytes.fromhex(previous["summary_hex"])).hexdigest()
        ):
            raise ValueError
        # Material policy was independently revalidated; a new default-branch
        # display-only commit may change policy_revision without invalidating it.


def main():
    try:
        raw = sys.stdin.buffer.read(26 * 1024 * 1024 + 1)
        if len(raw) > 26 * 1024 * 1024:
            raise ValueError
        verify(json.loads(raw))
    except Exception:
        print('{"code":"RUNNER_NOT_READY"}')
        return 1
    print('{"code":"VERIFIED"}')
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
