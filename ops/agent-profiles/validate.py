#!/usr/bin/env python3
"""AW-003: offline schema validation; no GitHub or Codex access."""

import argparse
import json
from pathlib import Path

from catalog import MAX_BYTES, CatalogError, parse_catalog
from diagnostics import diagnostic, schema_diagnostics
from jsonschema import Draft202012Validator

SCHEMA_PATH = Path(__file__).with_name("schema.v1.json")


def validate_catalog(content):
    try:
        catalog = parse_catalog(content)
    except CatalogError as error:
        return [diagnostic(error.code, "/", str(error))]
    if isinstance(catalog, dict) and type(catalog.get("schema_version")) is int:
        if catalog["schema_version"] != 1:
            return [
                diagnostic(
                    "UNSUPPORTED_VERSION", "/schema_version", "Use schema_version: 1."
                )
            ]
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    validator = Draft202012Validator(schema)
    return schema_diagnostics(validator.iter_errors(catalog))


def validate_file(path):
    try:
        with Path(path).open("rb") as stream:
            return validate_catalog(stream.read(MAX_BYTES + 1))
    except FileNotFoundError:
        return [diagnostic("MISSING_FILE", "/", "Add .github/agent-profiles.yml.")]
    except OSError:
        return [
            diagnostic("UNREADABLE_FILE", "/", "Check catalog file access and retry.")
        ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("catalog", nargs="?", default=".github/agent-profiles.yml")
    arguments = parser.parse_args()
    errors = validate_file(arguments.catalog)
    print(json.dumps({"valid": not errors, "errors": errors}, sort_keys=True))
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
