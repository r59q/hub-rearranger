"""Check workflow safety boundaries and the real intake/gate wiring offline."""

import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[2]


class WorkflowTests(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.safe_load(
            (ROOT / ".github/workflows/agent-assignment.yml").read_text()
        )

    def test_only_created_comments_with_stable_identity_can_enter(self):
        # PyYAML's YAML 1.1 loader represents the GitHub Actions 'on' key as True.
        self.assertEqual(self.workflow[True], {"issue_comment": {"types": ["created"]}})
        self.assertEqual(
            self.workflow["run-name"],
            "Agent assignment ${{ github.event.repository.id }}:"
            "${{ github.event.comment.id }}",
        )
        self.assertEqual(
            self.workflow["concurrency"],
            {
                "group": "agent-assignment-${{ github.event.repository.id }}-"
                "${{ github.event.comment.id }}",
                "cancel-in-progress": False,
            },
        )

    def test_authorization_and_reporting_are_hosted_and_least_privilege(self):
        self.assertEqual(self.workflow["permissions"], {})

        jobs = self.workflow["jobs"]
        self.assertEqual(
            jobs["authorize"]["permissions"],
            {"contents": "read", "actions": "read", "issues": "write"},
        )

        for name in ("authorize", "dispatch", "report"):
            job = jobs[name]
            self.assertEqual(job["runs-on"], "ubuntu-24.04")
            self.assertNotIn("secrets", job)

        self.assertEqual(
            jobs["dispatch"]["permissions"],
            {"contents": "read", "actions": "read", "issues": "read"},
        )
        self.assertEqual(
            jobs["dispatch"]["if"], "needs.authorize.outputs.dispatch == 'true'"
        )

    def test_trusted_checkout_and_no_comment_interpolation_into_shell(self):
        steps = self.workflow["jobs"]["authorize"]["steps"]
        self.assertEqual(
            steps[0]["with"],
            {"ref": "${{ github.workflow_sha }}", "persist-credentials": False},
        )

        for step in steps:
            if "uses" in step:
                revision = step["uses"].split("@")[-1]
                self.assertEqual(len(revision), 40)
            if "run" in step:
                self.assertNotIn("${{", step["run"])
                self.assertNotIn("comment.body", step["run"])

        intake = next(step for step in steps if step.get("id") == "intake")
        self.assertIn(
            '--policy "$GITHUB_WORKSPACE/ops/agent-intake/profile_policy.py"',
            intake["run"],
        )
        self.assertEqual(intake["env"], {"GH_TOKEN": "${{ github.token }}"})

        artifact = steps[-1]
        self.assertEqual(artifact["if"], "steps.intake.outputs.dispatch == 'true'")
        self.assertEqual(
            artifact["with"]["name"], "agent-invocation-v1-${{ github.run_attempt }}"
        )


if __name__ == "__main__":
    unittest.main()
