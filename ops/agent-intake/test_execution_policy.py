"""Offline canonical readiness/recovery checks used by both execution gates."""

import datetime
import hashlib
import unittest
from pathlib import Path

from execution_policy import verify
from jsonschema import ValidationError

ROOT = Path(__file__).resolve().parents[1]


def request():
    now = datetime.datetime(2026, 10, 2, 12, tzinfo=datetime.UTC)
    value = {
        "invocation": {
            "repository": {"owner": "octo", "name": "demo"},
            "profile_id": "codex-thorough",
            "policy_revision": "2" * 40,
            "profile": {
                "model": {
                    "id": "gpt-6.1-sol",
                    "reasoning_effort": "high",
                    "fallback": "none",
                },
                "authority": {
                    "mode": "branch-draft-pr",
                    "sandbox": "workspace-write",
                    "network": False,
                },
            },
        },
        "readiness": {
            "version": 1,
            "repository": "octo/demo",
            "profile_id": "codex-thorough",
            "profile_revision": "2" * 40,
            "runner_label": "hub-agent-codex",
            "requested_model": "gpt-6.1-sol",
            "effective_model": "gpt-6.1-sol",
            "requested_reasoning_effort": "high",
            "effective_reasoning_effort": "high",
            "cli_version": "codex-cli 0.159.3",
            "run_id": 99,
            "run_attempt": 1,
            "verified_at": now.isoformat(),
            "outcome": "verified",
            "reason_code": "verified",
        },
    }
    return value, now


class ExecutionPolicyTests(unittest.TestCase):
    def recovery(self):
        value, now = request()
        input = value["invocation"]
        input.update(
            {
                "assignment_id": "42:99",
                "operation": "assignment",
                "profile_revision": "1" * 40,
                "base_sha": "2" * 40,
                "run_id": 100,
                "run_attempt": 2,
            }
        )
        check = {
            "id": "repository-check",
            "outcome": "passed",
            "evidence": "Repository check passed.",
        }
        result = {
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
                )
            },
            "contract_version": 1,
            "run_attempt": 1,
            "head_sha": "2" * 40,
            "outcome": "ready",
            "reason_code": "PROPOSAL_READY",
            "message": "A patch is available for independent draft-PR publication.",
            "next_action": (
                "Review the original workflow attempt and its verified artifacts."
            ),
            "policy": {
                "requested_model": "gpt-6.1-sol",
                "effective_model": "gpt-6.1-sol",
                "requested_reasoning_effort": "high",
                "effective_reasoning_effort": "high",
                "cli_version": "codex-cli 0.159.3",
                "authority": "branch-draft-pr",
                "sandbox": "workspace-write",
                "network": False,
                "isolation": "verified",
            },
            "validation": [check],
            "proposal": {
                "patch": "proposal.patch",
                "patch_sha256": hashlib.sha256(b"patch").hexdigest(),
                "summary": "summary.json",
                "summary_sha256": hashlib.sha256(b"summary").hexdigest(),
            },
        }
        value["previous"] = {
            "result": result,
            "patch_hex": b"patch".hex(),
            "summary_hex": b"summary".hex(),
        }
        return value, now

    def test_prior_ready_proposal_can_be_reused_after_current_verification(self):
        value, now = self.recovery()
        verify(value, now)

    def test_recovery_rejects_forged_identity_digest_and_effective_policy(self):
        for variant in ("base", "attempt", "head", "patch", "summary", "model"):
            with self.subTest(variant=variant):
                value, now = self.recovery()
                result = value["previous"]["result"]
                if variant in ("patch", "summary"):
                    value["previous"][variant + "_hex"] = b"forged".hex()
                elif variant == "base":
                    result["base_sha"] = "3" * 40
                elif variant == "head":
                    result["head_sha"] = "3" * 40
                elif variant == "attempt":
                    result["run_attempt"] = 2
                else:
                    result["policy"]["effective_model"] = "other"
                with self.assertRaises((ValueError, ValidationError)):
                    verify(value, now)

    def test_current_matching_evidence_is_accepted(self):
        value, now = request()
        verify(value, now)

    def test_stale_failed_fallback_or_mismatched_evidence_is_rejected(self):
        for field, replacement in {
            "verified_at": "2026-09-30T12:00:00+00:00",
            "repository": "unrelated/repo",
            "profile_revision": "3" * 40,
            "effective_model": "other",
            "runner_label": "addons",
            "outcome": "failed",
            "effective_reasoning_effort": "low",
            "credential": "synthetic-sentinel",
        }.items():
            with self.subTest(field=field):
                value, now = request()
                value["readiness"][field] = replacement
                with self.assertRaises((ValueError, ValidationError)):
                    verify(value, now)

    def test_future_evidence_is_rejected(self):
        value, now = request()
        with self.assertRaises(ValueError):
            verify(value, now - datetime.timedelta(seconds=1))


if __name__ == "__main__":
    unittest.main()
