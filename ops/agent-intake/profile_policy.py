#!/usr/bin/env python3
"""Validate both catalogs using AW-003; emit execution policy, never source prose."""

import base64
import binascii
import json
import sys
from pathlib import Path

# This is reviewed intake code, not an assignment-selected import path.
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "agent-profiles"))

from catalog import parse_catalog  # noqa: E402
from validate import validate_catalog  # noqa: E402


def verify(pinned, current, profile_id):
    if validate_catalog(pinned) or validate_catalog(current):
        return {"code": "PROFILE_INVALID"}

    entries = [
        parse_catalog(content)["profiles"].get(profile_id)
        for content in (pinned, current)
    ]
    if any(entry is None for entry in entries):
        return {"code": "PROFILE_INVALID"}
    if any(not entry["enabled"] for entry in entries):
        return {"code": "PROFILE_DISABLED"}

    policies = []
    for entry in entries:
        policy = {
            key: value
            for key, value in entry.items()
            if key not in {"name", "description", "enabled"}
        }
        policy["context"] = {
            **policy["context"],
            "sources": sorted(policy["context"]["sources"]),
        }
        policy["validation"] = {
            **policy["validation"],
            "checks": sorted(policy["validation"]["checks"]),
        }
        policies.append(policy)

    if policies[0] != policies[1]:
        return {"code": "PROFILE_POLICY_CHANGED"}

    # v1 dispatch is a reviewed fixed adapter/label, never a profile-selected runner.
    if policies[0]["adapter"] != {
        "id": "codex-chatgpt-private-runner",
        "contract_version": 1,
        "runner_label": "hub-agent-codex",
    }:
        return {"code": "PROFILE_POLICY_CHANGED"}

    return {"code": "ACCEPTED", "profile": policies[0]}


def main():
    try:
        raw = sys.stdin.buffer.read(200_001)
        if len(raw) > 200_000:
            raise ValueError

        request = json.loads(raw)
        result = verify(
            base64.b64decode(request["pinned"], validate=True),
            base64.b64decode(request["current"], validate=True),
            request["profile_id"],
        )
    except (ValueError, KeyError, TypeError, binascii.Error):
        result = {"code": "PROFILE_INVALID"}

    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
