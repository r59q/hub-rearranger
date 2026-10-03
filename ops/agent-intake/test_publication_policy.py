"""AW-012 producer/AW-013 consumer contract and tamper checks, offline."""

import copy
import json
import sys
import unittest
from pathlib import Path

from jsonschema import ValidationError
from publication_policy import verify

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "private-runner"))

from execution_result import REASONS, result


def request(outcome="passed"):
    origin = {
        "assignment_id": "42:99",
        "operation": "assignment",
        "profile_id": "codex-thorough",
        "profile_revision": "1" * 40,
        "policy_revision": "2" * 40,
        "base_sha": "2" * 40,
        "run_id": 100,
        "run_attempt": 1,
        "profile": {
            "validation": {
                "checks": ["repository-check"],
                "on_failure": "draft-with-evidence",
            }
        },
    }
    evidence = {
        "passed": "Repository check passed.",
        "failed": "Repository check failed; inspect privately.",
        "unavailable": "Repository check unavailable in the offline sandbox.",
        "cancelled": "Repository check cancelled.",
    }[outcome]
    checks = [{"id": "repository-check", "outcome": outcome, "evidence": evidence}]
    patch = b"synthetic-patch"
    summary = json.dumps(
        {
            "assignment_id": origin["assignment_id"],
            "summary": REASONS["PROPOSAL_READY"],
            "validation": checks,
        }
    ).encode()
    record = result(origin, "PROPOSAL_READY", checks, True, patch, summary)
    return {
        "invocation": {**origin, "run_attempt": 2},
        "origin": origin,
        "result": record,
        "attempt": 1,
        "patch_hex": patch.hex(),
        "summary_hex": summary.hex(),
    }


class PublicationPolicyTests(unittest.TestCase):
    def test_actual_producer_contract_accepts_honest_validation(self):
        for outcome in ("passed", "failed", "unavailable"):
            with self.subTest(outcome=outcome):
                verify(request(outcome))

    def test_forged_identity_artifacts_and_policy_fail_closed(self):
        for variant in (
            "assignment",
            "profile",
            "base",
            "run",
            "attempt",
            "future",
            "origin",
            "policy_revision",
            "model",
            "network",
            "isolation",
            "patch",
            "summary",
            "unknown",
            "cancelled",
        ):
            with self.subTest(variant=variant):
                value = copy.deepcopy(request())
                record = value["result"]
                if variant in {"assignment", "profile", "base", "run"}:
                    key = {
                        "assignment": "assignment_id",
                        "profile": "profile_revision",
                        "base": "base_sha",
                        "run": "run_id",
                    }[variant]
                    record[key] = (
                        "3" * 40
                        if variant in {"profile", "base"}
                        else ("42:100" if variant == "assignment" else 101)
                    )
                elif variant == "attempt":
                    value["attempt"] = 2
                elif variant == "future":
                    value["invocation"]["run_attempt"] = 0
                elif variant == "origin":
                    value["origin"]["base_sha"] = "3" * 40
                elif variant == "policy_revision":
                    record["policy_revision"] = "3" * 40
                elif variant == "model":
                    record["policy"]["effective_model"] = None
                elif variant == "network":
                    record["policy"]["network"] = True
                elif variant == "isolation":
                    record["policy"]["isolation"] = "unverified"
                elif variant == "patch":
                    value["patch_hex"] = b"forged".hex()
                elif variant == "summary":
                    value["summary_hex"] = b"forged".hex()
                elif variant == "unknown":
                    record["command"] = "private-sentinel"
                else:
                    value = request("cancelled")
                with self.assertRaises((ValueError, ValidationError)):
                    verify(value)
