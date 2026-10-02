#!/usr/bin/env python3
"""Verify AW-012 result identity and canonical schemas for separate publication."""

import hashlib
import json
import sys
from pathlib import Path

from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[1] / "private-runner"


def verify(request):
    input = request["invocation"]
    result = request["result"]
    origin = request["origin"]
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
    if any(result[key] != input[key] or origin[key] != input[key] for key in keys):
        raise ValueError
    if result["outcome"] != "ready" or result["reason_code"] != "PROPOSAL_READY":
        raise ValueError
    if (
        not 0
        < result["run_attempt"]
        == origin["run_attempt"]
        == request["attempt"]
        <= input["run_attempt"]
    ):
        raise ValueError
    if (
        result["policy_revision"] != origin["policy_revision"]
        or result["head_sha"] != input["base_sha"]
    ):
        raise ValueError
    # Both originating and current profiles were independently checked against
    # canonical catalogs. Artifact metadata cannot define a separate policy.
    if origin["profile"] != input["profile"]:
        raise ValueError
    patch = bytes.fromhex(request["patch_hex"])
    summary = bytes.fromhex(request["summary_hex"])
    if not 0 < len(patch) <= 8 * 1024 * 1024 or len(summary) > 65536:
        raise ValueError
    if (
        result["proposal"]["patch_sha256"] != hashlib.sha256(patch).hexdigest()
        or result["proposal"]["summary_sha256"] != hashlib.sha256(summary).hexdigest()
    ):
        raise ValueError
    expected = {
        "assignment_id": input["assignment_id"],
        "summary": result["message"],
        "validation": result["validation"],
    }
    if json.loads(summary) != expected:
        raise ValueError
    for check in result["validation"]:
        if check["outcome"] == "cancelled":
            raise ValueError
    if input["profile"]["validation"] != {
        "checks": ["repository-check"],
        "on_failure": "draft-with-evidence",
    }:
        raise ValueError


def main():
    try:
        payload = sys.stdin.buffer.read(26 * 1024 * 1024 + 1)
        if len(payload) > 26 * 1024 * 1024:
            raise ValueError
        verify(json.loads(payload))
    except Exception:
        print('{"code":"REPLAY_STATE_UNAVAILABLE"}')
        return 1
    print('{"code":"VERIFIED"}')
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
