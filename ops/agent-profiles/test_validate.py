"""Offline profile contract tests; fixtures contain no live repository data."""

import copy
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml
from catalog import MAX_BYTES, MAX_DEPTH
from jsonschema import Draft202012Validator
from validate import SCHEMA_PATH, validate_catalog, validate_file

ROOT = Path(__file__).resolve().parents[2]
VALID_CATALOG = (ROOT / ".github/agent-profiles.yml").read_bytes()


def altered_catalog(field, value):
    catalog = yaml.safe_load(VALID_CATALOG)
    target = catalog["profiles"]["codex-thorough"]
    for part in field[:-1]:
        target = target[part]
    target[field[-1]] = value
    return yaml.safe_dump(catalog).encode()


class CatalogValidationTests(unittest.TestCase):
    def test_schema_is_valid_draft_2020_12(self):
        schema = json.loads(SCHEMA_PATH.read_text())

        Draft202012Validator.check_schema(schema)

    def test_repository_codex_thorough_profile_is_valid(self):
        errors = validate_catalog(VALID_CATALOG)

        self.assertEqual(errors, [])

    def test_unknown_model_is_configuration_not_runtime_verification(self):
        content = altered_catalog(["model", "id"], "future-model")

        errors = validate_catalog(content)

        self.assertEqual(errors, [])

    def test_missing_file_gives_actionable_state(self):
        with tempfile.TemporaryDirectory() as directory:
            errors = validate_file(Path(directory) / "missing.yml")

        self.assertEqual(errors[0]["code"], "MISSING_FILE")
        self.assertEqual(errors[0]["message"], "Add .github/agent-profiles.yml.")

    def test_unsupported_version_is_distinct_from_invalid_fields(self):
        content = b"schema_version: 2\nprofiles: {}"

        errors = validate_catalog(content)

        self.assertEqual(len(errors), 1)
        self.assertEqual(errors[0]["code"], "UNSUPPORTED_VERSION")
        self.assertEqual(errors[0]["path"], "/schema_version")

    def test_invalid_yaml_does_not_echo_source_or_parser_exception(self):
        for content in [
            b"private-sentinel: [unterminated",
            b"schema_version: 1\n---\nprivate-sentinel: true",
            b"!!python/object:private-sentinel {}",
            b"private-sentinel: \xff",
            b"[private-sentinel]: value",
        ]:
            with self.subTest(content=content):
                errors = validate_catalog(content)

                self.assertEqual(errors[0]["code"], "INVALID_YAML")
                self.assertNotIn("private-sentinel", json.dumps(errors))

    def test_empty_and_non_mapping_catalogs_are_invalid(self):
        for content in [b"", b"null", b"[]", b"hello"]:
            with self.subTest(content=content):
                errors = validate_catalog(content)

                self.assertEqual(errors[0]["code"], "INVALID_TYPE")

    def test_duplicate_keys_cannot_silently_override_authority(self):
        content = VALID_CATALOG.replace(
            b"mode: branch-draft-pr",
            b"mode: private-sentinel\n      mode: branch-draft-pr",
        )

        errors = validate_catalog(content)

        self.assertEqual(errors[0]["code"], "DUPLICATE_KEY")
        self.assertNotIn("private-sentinel", json.dumps(errors))

    def test_aliases_anchors_and_merge_keys_are_rejected(self):
        for content in [
            b"profiles: &base {}",
            b"profiles: *base",
            b"profiles: {<<: {}}",
        ]:
            with self.subTest(content=content):
                errors = validate_catalog(content)

                self.assertEqual(errors[0]["code"], "INVALID_YAML")

    def test_size_and_depth_limits_are_checked_before_schema_validation(self):
        for content in [
            b" " * (MAX_BYTES + 1),
            b"[" * (MAX_DEPTH + 1) + b"]" * (MAX_DEPTH + 1),
        ]:
            with self.subTest(size=len(content)):
                errors = validate_catalog(content)

                self.assertEqual(errors[0]["code"], "LIMIT_EXCEEDED")

    def test_size_and_depth_limits_include_the_boundary(self):
        for content in [b" " * MAX_BYTES, b"[" * MAX_DEPTH + b"]" * MAX_DEPTH]:
            with self.subTest(size=len(content)):
                errors = validate_catalog(content)

                self.assertEqual(errors[0]["code"], "INVALID_TYPE")

    def test_boolean_spelling_is_explicit_and_does_not_change_safe_loader(self):
        for spelling in [b"yes", b"on", b"True"]:
            content = VALID_CATALOG.replace(b"enabled: true", b"enabled: " + spelling)

            errors = validate_catalog(content)

            self.assertEqual(errors[0]["code"], "INVALID_TYPE")
        self.assertIs(yaml.safe_load("on"), True)

    def test_missing_fields_have_stable_paths_and_recovery(self):
        catalog = yaml.safe_load(VALID_CATALOG)
        del catalog["profiles"]["codex-thorough"]["model"]["id"]

        errors = validate_catalog(yaml.safe_dump(catalog).encode())

        self.assertEqual(
            errors,
            [
                {
                    "code": "MISSING_FIELD",
                    "path": "/profiles/<profile>/model/id",
                    "message": "Add this required field.",
                }
            ],
        )

    def test_credentials_commands_and_runtime_overrides_are_not_schema_fields(self):
        catalog = yaml.safe_load(VALID_CATALOG)
        for field in [
            "credentials",
            "token",
            "command",
            "env",
            "cli_flags",
            "revision",
        ]:
            for location in [[], ["adapter"], ["model"], ["authority"], ["validation"]]:
                with self.subTest(field=field, location=location):
                    target = copy.deepcopy(catalog)
                    mapping = target["profiles"]["codex-thorough"]
                    for part in location:
                        mapping = mapping[part]
                    mapping[field] = "private-sentinel"

                    errors = validate_catalog(yaml.safe_dump(target).encode())

                    self.assertIn(
                        "UNSUPPORTED_FIELD", {error["code"] for error in errors}
                    )
                    self.assertNotIn("private-sentinel", json.dumps(errors))

    def test_unknown_root_fields_are_rejected(self):
        content = VALID_CATALOG + b"private-sentinel: secret\n"

        errors = validate_catalog(content)

        self.assertEqual(errors[0]["code"], "UNSUPPORTED_FIELD")
        self.assertNotIn("private-sentinel", json.dumps(errors))

    def test_error_paths_do_not_disclose_profile_ids_or_values(self):
        catalog = yaml.safe_load(VALID_CATALOG)
        profile = catalog["profiles"].pop("codex-thorough")
        profile["model"]["reasoning_effort"] = "private-sentinel-value"
        catalog["profiles"]["private-sentinel-id"] = profile

        errors = validate_catalog(yaml.safe_dump(catalog).encode())

        self.assertNotIn("private-sentinel", json.dumps(errors))
        self.assertEqual(
            errors[0]["path"], "/profiles/<profile>/model/reasoning_effort"
        )

    def test_unsupported_adapter_authority_and_deferred_capabilities_fail_closed(self):
        for field, value in [
            (["adapter", "id"], "codex-cloud"),
            (["adapter", "contract_version"], 2),
            (["adapter", "runner_label"], "self-hosted"),
            (["adapter", "runner_label"], "linux"),
            (["model", "id"], "$(private-sentinel)"),
            (["model", "fallback"], "automatic"),
            (["model", "reasoning_effort"], "ultra"),
            (["authority", "mode"], "merge"),
            (["authority", "sandbox"], "danger-full-access"),
            (["authority", "network"], True),
            (["context", "images"], True),
            (["continuation", "pipeline"], "remediate"),
            (["validation", "checks"], ["make check"]),
            (["triggers", "assignment"], "pull_request_target"),
        ]:
            with self.subTest(field=field):
                errors = validate_catalog(altered_catalog(field, value))

                self.assertTrue(errors)

    def test_false_constants_do_not_accept_numeric_zero(self):
        content = altered_catalog(["authority", "network"], 0)

        errors = validate_catalog(content)

        self.assertIn("INVALID_TYPE", {error["code"] for error in errors})

    def test_review_continuation_requires_current_pr_thread_and_checks_context(self):
        content = altered_catalog(["context", "sources"], ["issue", "repository"])

        errors = validate_catalog(content)

        self.assertEqual(len(errors), 3)
        self.assertEqual({error["code"] for error in errors}, {"MISSING_CONTEXT"})
        self.assertEqual(
            {error["message"] for error in errors},
            {
                "Include context source: pull_request.",
                "Include context source: review_thread.",
                "Include context source: checks.",
            },
        )
        self.assertEqual(errors[0]["path"], "/profiles/<profile>/context/sources")

    def test_issue_only_profile_can_disable_continuation(self):
        catalog = yaml.safe_load(VALID_CATALOG)
        profile = catalog["profiles"]["codex-thorough"]
        profile["continuation"]["review_comments"] = False
        profile["context"]["sources"] = ["issue", "repository"]
        profile["enabled"] = False

        errors = validate_catalog(yaml.safe_dump(catalog).encode())

        self.assertEqual(errors, [])

    def test_duplicate_context_is_invalid(self):
        sources = yaml.safe_load(VALID_CATALOG)["profiles"]["codex-thorough"][
            "context"
        ]["sources"]

        errors = validate_catalog(
            altered_catalog(["context", "sources"], sources + ["issue"])
        )

        self.assertIn("DUPLICATE_VALUE", {error["code"] for error in errors})

    def test_diagnostics_are_independent_of_mapping_order(self):
        catalog = yaml.safe_load(
            altered_catalog(["authority", "mode"], "private-sentinel")
        )
        del catalog["profiles"]["codex-thorough"]["model"]

        ordered = validate_catalog(yaml.safe_dump(catalog, sort_keys=True).encode())
        reversed_order = validate_catalog(
            yaml.safe_dump(catalog, sort_keys=False).encode()
        )

        self.assertEqual(ordered, reversed_order)
        self.assertEqual(
            ordered, sorted(ordered, key=lambda item: (item["path"], item["code"]))
        )

    def test_cli_publishes_only_validation_status_and_safe_diagnostics(self):
        for content, expected_status in [
            (VALID_CATALOG, 0),
            (b"private-sentinel: [", 1),
        ]:
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "catalog.yml"
                path.write_bytes(content)

                result = subprocess.run(
                    [
                        sys.executable,
                        str(SCHEMA_PATH.with_name("validate.py")),
                        str(path),
                    ],
                    capture_output=True,
                    text=True,
                    check=False,
                )

            self.assertEqual(result.returncode, expected_status)
            self.assertEqual(result.stderr, "")
            self.assertNotIn("private-sentinel", result.stdout)
            self.assertEqual(json.loads(result.stdout)["valid"], expected_status == 0)


if __name__ == "__main__":
    unittest.main()
