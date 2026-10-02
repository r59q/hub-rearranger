#!/usr/bin/env python3
"""Export the normative schema for Go embedding and typed OpenAPI transport."""

import argparse
import copy
import json
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[2]
SCHEMA = Path(__file__).with_name("schema.v1.json")
EMBEDDED = ROOT / "services/agents/internal/infrastructure/profiles/schema.v1.gen.json"
FRONTEND_SCHEMA = ROOT / "frontend/src/lib/server/profile-schema.gen.json"
EVIDENCE = ROOT / "ops/private-runner/readiness.schema.v1.json"
EVIDENCE_EMBEDDED = (
    ROOT / "services/agents/internal/infrastructure/evidence/schema.v1.gen.json"
)
EVIDENCE_FRONTEND = ROOT / "frontend/src/lib/server/evidence-schema.gen.json"
OPENAPI = ROOT / "services/agents/api/openapi.yaml"


def transport_schema(value):
    """Project field shapes to OpenAPI 3.0; full validation uses the original."""
    if isinstance(value, list):
        return [transport_schema(item) for item in value]
    if not isinstance(value, dict):
        return value

    result = {}
    for key, item in value.items():
        # These JSON Schema keywords have no OpenAPI 3.0 equivalent. Removing
        # patterns also avoids Go/ECMAScript regexp differences in API validators.
        if key in {"allOf", "contains", "if", "then", "else", "pattern"}:
            continue
        if key == "const":
            result["enum"] = [item]
        elif key == "type" and isinstance(item, list) and "null" in item:
            result["type"] = next(kind for kind in item if kind != "null")
            result["nullable"] = True
        elif key == "enum" and isinstance(item, list):
            result[key] = [entry for entry in item if entry is not None]
        elif key == "$ref":
            result[key] = (
                "#/components/schemas/Catalog"
                + item.removeprefix("#/$defs/").capitalize()
            )
        else:
            result[key] = transport_schema(item)
    return result


def exports(schema):
    result = {}
    for name, definition in schema["$defs"].items():
        projected = transport_schema(definition)
        # Keep names stable across the OpenAPI generators.
        result["Catalog" + name.capitalize()] = projected
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()

    content = SCHEMA.read_bytes()
    generated = exports(json.loads(content))

    evidence_content = EVIDENCE.read_bytes()
    evidence_schema = json.loads(evidence_content)
    generated["RuntimeEvidence"] = transport_schema(
        {
            key: value
            for key, value in evidence_schema.items()
            if key not in {"$schema", "$id", "title"}
        }
    )

    document = yaml.safe_load(OPENAPI.read_text())
    schemas = document["components"]["schemas"]
    existing = {
        key: value
        for key, value in schemas.items()
        if key.startswith("Catalog") or key == "RuntimeEvidence"
    }

    if args.check:
        if (
            not EMBEDDED.exists()
            or EMBEDDED.read_bytes() != content
            or not FRONTEND_SCHEMA.exists()
            or json.loads(FRONTEND_SCHEMA.read_bytes()) != json.loads(content)
            or existing != generated
            or not EVIDENCE_EMBEDDED.exists()
            or EVIDENCE_EMBEDDED.read_bytes() != evidence_content
            or not EVIDENCE_FRONTEND.exists()
            or json.loads(EVIDENCE_FRONTEND.read_bytes()) != evidence_schema
        ):
            print("Agents contracts are stale; run make generate-agents-contract.")
            return 1
        return 0

    EVIDENCE_EMBEDDED.write_bytes(evidence_content)
    EVIDENCE_FRONTEND.write_bytes(evidence_content)
    EMBEDDED.write_bytes(content)
    FRONTEND_SCHEMA.write_bytes(content)
    for key in existing:
        del schemas[key]
    schemas.update(copy.deepcopy(generated))
    OPENAPI.write_text(yaml.safe_dump(document, sort_keys=False, width=100))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
