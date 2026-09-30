"""Stable diagnostics built from schema keywords, never raw parser messages."""

FIELD_NAMES = frozenset(
    "schema_version profiles enabled name description role adapter model triggers "
    "context authority validation continuation id contract_version runner_label "
    "reasoning_effort fallback assignment sources images mode sandbox network "
    "checks on_failure review_comments pipeline".split()
)


def diagnostic(code, path, message):
    return {"code": code, "path": path, "message": message}


def safe_path(parts):
    result = []
    for index, part in enumerate(parts):
        if isinstance(part, int):
            result.append(str(part))
        elif index == 1 and parts[0] == "profiles":
            # Profile IDs are data, not trusted diagnostic strings. Redact even
            # syntactically valid IDs so misplaced credentials cannot be echoed.
            result.append("<profile>")
        elif part in FIELD_NAMES:
            result.append(part)
        else:
            result.append("<field>")
    return "/" + "/".join(result) if result else "/"


def schema_diagnostics(errors):
    result = []
    for error in errors:
        path = list(error.absolute_path)
        keyword = error.validator
        if keyword == "required":
            for field in error.validator_value:
                if field not in error.instance:
                    result.append(
                        diagnostic(
                            "MISSING_FIELD",
                            safe_path([*path, field]),
                            "Add this required field.",
                        )
                    )
            continue
        code, message = {
            "type": ("INVALID_TYPE", "Use the field type defined in the v1 schema."),
            "additionalProperties": (
                "UNSUPPORTED_FIELD",
                "Remove fields not defined in the v1 schema.",
            ),
            "const": (
                "UNSUPPORTED_VALUE",
                "Use the supported value defined in the v1 schema.",
            ),
            "enum": (
                "UNSUPPORTED_VALUE",
                "Choose a supported value from the v1 schema.",
            ),
            "pattern": ("INVALID_FORMAT", "Use the format defined in the v1 schema."),
            "uniqueItems": ("DUPLICATE_VALUE", "Remove duplicate list values."),
            "contains": (
                "MISSING_CONTEXT",
                "Include the context sources required by this policy.",
            ),
        }.get(
            keyword, ("LIMIT_EXCEEDED", "Use the size limits defined in the v1 schema.")
        )
        # Only schema-owned expected values may appear in recovery guidance.
        if keyword == "type":
            message = f"Use type: {error.validator_value}."
        elif keyword == "const":
            message = f"Use value: {str(error.validator_value).lower()}."
        elif keyword == "enum":
            message = f"Choose from: {', '.join(error.validator_value)}."
        elif keyword == "contains":
            message = f"Include context source: {error.validator_value['const']}."
        result.append(diagnostic(code, safe_path(path), message))
    # Multiple contains failures can share a path. Sort and deduplicate for a
    # deterministic API/CLI response, independent of YAML mapping order.
    unique = {(item["path"], item["code"], item["message"]) for item in result}
    return [diagnostic(code, path, message) for path, code, message in sorted(unique)]
