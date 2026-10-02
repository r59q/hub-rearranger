"""Adapter-owned results: fixed prose, no model-authored metadata or raw output."""

import hashlib
import json

from execution_config import CLI_VERSION, EFFORT, MODEL

REASONS = {
    "PROPOSAL_READY": "A patch is available for independent draft-PR publication.",
    "RUNNER_NOT_READY": "Verify the dedicated runner, isolation, and runtime evidence.",
    "AUTHORIZATION_DENIED": "Restore current repository access before retrying.",
    "REPLAY_STATE_UNAVAILABLE": "Reconcile the original execution before retrying.",
    "MODEL_POLICY_UNSUPPORTED": (
        "Verify the exact model and reasoning policy without fallback."
    ),
    "NO_CHANGES": "The assignment produced no source changes.",
    "PROTECTED_CHANGE": (
        "The proposed changes exceed the adapter protected-path boundary."
    ),
    "CANCELLED": (
        "Execution was cancelled; reconcile the original attempt before retrying."
    ),
    "RUNTIME_FAILED": (
        "Execution failed; inspect runner setup privately "
        "and reconcile before retrying."
    ),
    "RUNTIME_TIMEOUT": "Execution exceeded its deadline; reconcile before retrying.",
    "OUTPUT_LIMIT_EXCEEDED": "Execution exceeded its output limit.",
    "SOURCE_LIMIT_EXCEEDED": "The checkout exceeds the supported source limits.",
    "UNSAFE_SOURCE": "The checkout contains unsupported paths or file types.",
    "PATCH_UNAVAILABLE": "The adapter could not safely capture a patch.",
    "PATCH_LIMIT_EXCEEDED": "The proposal exceeds the supported patch limit.",
}


def result(input, code, checks, effective=False, patch=None, summary=None):
    value = {
        "contract_version": 1,
        **{
            key: input[key]
            for key in (
                "assignment_id",
                "operation",
                "profile_id",
                "profile_revision",
                "policy_revision",
                "base_sha",
                "run_id",
                "run_attempt",
            )
        },
        "head_sha": input["base_sha"],
        "outcome": "ready"
        if code == "PROPOSAL_READY"
        else (
            "cancelled"
            if code == "CANCELLED"
            else (
                "blocked"
                if code in {"NO_CHANGES", "PROTECTED_CHANGE", "RUNNER_NOT_READY"}
                else "failed"
            )
        ),
        "reason_code": code,
        "message": REASONS[code],
        "next_action": (
            "Review the original workflow attempt and its verified artifacts."
        ),
        "policy": {
            "requested_model": MODEL,
            "requested_reasoning_effort": EFFORT,
            "effective_model": MODEL if effective else None,
            "effective_reasoning_effort": EFFORT if effective else None,
            "cli_version": CLI_VERSION,
            "authority": "branch-draft-pr",
            "sandbox": "workspace-write",
            "network": False,
            "isolation": "verified" if effective else "unverified",
        },
        "validation": checks,
        "proposal": None,
    }
    if patch is not None:
        value["proposal"] = {
            "patch": "proposal.patch",
            "patch_sha256": hashlib.sha256(patch).hexdigest(),
            "summary": "summary.json",
            "summary_sha256": hashlib.sha256(summary).hexdigest(),
        }
    return value


def publish(directory, input, code, checks, effective=False, patch=None):
    directory.mkdir(mode=0o700, exist_ok=True)
    summary = json.dumps(
        {
            "assignment_id": input["assignment_id"],
            "summary": REASONS[code],
            "validation": checks,
        },
        sort_keys=True,
    ).encode()
    value = result(input, code, checks, effective, patch, summary)
    # Only proposal-ready outputs contain repository material. Failed/cancelled
    # outputs cannot accidentally upload partial changes or provider transcripts.
    if patch is not None:
        (directory / "proposal.patch").write_bytes(patch)
        (directory / "summary.json").write_bytes(summary)
    (directory / "result.json").write_text(json.dumps(value, sort_keys=True))
    for path in directory.iterdir():
        path.chmod(0o600)
    return value
