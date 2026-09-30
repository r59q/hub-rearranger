"""Parse the bounded YAML subset used by repository profile catalogs."""

import re

import yaml

MAX_BYTES = 65_536
MAX_DEPTH = 24


class CatalogError(Exception):
    """Only fixed, credential-safe diagnostics may be attached to this error."""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code


class CatalogLoader(yaml.SafeLoader):
    """Use SafeLoader hooks to reject ambiguous or excessively nested YAML."""

    def __init__(self, stream):
        super().__init__(stream)
        self.depth = 0

    def compose_node(self, parent, index):
        event = self.peek_event()
        if isinstance(event, yaml.AliasEvent) or getattr(event, "anchor", None):
            raise CatalogError(
                "INVALID_YAML", "Remove YAML anchors and aliases; use explicit values."
            )
        if getattr(event, "tag", None):
            raise CatalogError("INVALID_YAML", "Remove explicit YAML tags.")
        self.depth += 1
        try:
            if self.depth > MAX_DEPTH:
                raise CatalogError(
                    "LIMIT_EXCEEDED", "Reduce catalog nesting to 24 levels."
                )
            return super().compose_node(parent, index)
        finally:
            self.depth -= 1

    def construct_mapping(self, node, deep=False):
        if not isinstance(node, yaml.MappingNode):
            raise CatalogError("INVALID_YAML", "Use a mapping for catalog fields.")
        mapping = {}
        for key_node, value_node in node.value:
            key = self.construct_object(key_node, deep=deep)
            if not isinstance(key, str):
                raise CatalogError(
                    "INVALID_YAML", "Use string keys for catalog fields."
                )
            if key in mapping:
                raise CatalogError("DUPLICATE_KEY", "Remove duplicate mapping keys.")
            mapping[key] = self.construct_object(value_node, deep=deep)
        return mapping


# PyYAML defaults to YAML 1.1. Keep on/off/yes/no and dates as strings;
# catalog booleans must be the explicit lowercase true/false spellings.
CatalogLoader.yaml_implicit_resolvers = {
    key: [
        (tag, pattern)
        for tag, pattern in resolvers
        if tag not in {"tag:yaml.org,2002:bool", "tag:yaml.org,2002:timestamp"}
    ]
    for key, resolvers in yaml.SafeLoader.yaml_implicit_resolvers.items()
}
CatalogLoader.add_implicit_resolver(
    "tag:yaml.org,2002:bool", re.compile(r"^(?:true|false)$"), ["t", "f"]
)


def parse_catalog(content):
    if len(content) > MAX_BYTES:
        raise CatalogError(
            "LIMIT_EXCEEDED", "Reduce the catalog to at most 65536 bytes."
        )
    try:
        text = content.decode("utf-8")
        return yaml.load(text, Loader=CatalogLoader)
    except (UnicodeError, yaml.YAMLError, ValueError, RecursionError):
        # Parser exceptions can quote source text, including misplaced secrets.
        raise CatalogError(
            "INVALID_YAML", "Use one UTF-8 YAML document with supported scalar values."
        ) from None
