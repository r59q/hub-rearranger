"""Fixed runner configuration and sandbox policy; never profile-selected code."""

import hashlib
import json
import os
import stat
from pathlib import Path

RUNTIME = Path.home() / ".local/lib/hub-agent-execution"
MANIFEST = RUNTIME / "operator.json"
AUTH = Path.home() / ".codex"
WORK = Path.home() / ".local/state/hub-agent-execution"
MODEL = "gpt-6.1-sol"
EFFORT = "high"
CLI_VERSION = "codex-cli 0.159.3"


REQUIRED_FILES = {
    "diagnose.py",
    "profile_diagnostic.py",
    "execution_setup.py",
    "codex",
    "codex-code-mode-host",
    "agent-patch",
    "execution.py",
    "execution_config.py",
    "execution_process.py",
    "execution_workspace.py",
    "execution_result.py",
    "isolation_probe.py",
    "result.schema.v1.json",
    "ops/agent-intake/profile_policy.py",
    "ops/agent-intake/execution_policy.py",
    "ops/agent-profiles/catalog.py",
    "ops/agent-profiles/validate.py",
    "ops/agent-profiles/diagnostics.py",
    "ops/agent-profiles/schema.v1.json",
    "ops/private-runner/readiness.schema.v1.json",
    "ops/private-runner/result.schema.v1.json",
}


class ExecutionFailure(Exception):
    """Only a fixed safe reason code crosses the reporting boundary."""


def fail(code):
    raise ExecutionFailure(code)


def trusted(path):
    """Only the operator account/root may replace installed runtime files.

    The operator approved shared addons for r59q/hub-rearranger on 2026-10-02.
    Other jobs under this account remain part of that explicit trust boundary.
    Workload tools cannot access this installation through their sandbox.
    """
    path = path.resolve(strict=True)
    for item in (path, *path.parents):
        info = item.stat()
        if info.st_uid not in {0, os.getuid()} or info.st_mode & (
            stat.S_IWGRP | stat.S_IWOTH
        ):
            fail("RUNNER_NOT_READY")
    return path


def operator_config(environment, require_enabled=True):
    manifest = json.loads(trusted(MANIFEST).read_text())
    if (
        set(manifest)
        != {"version", "enabled", "repository", "runner_name", "cli_version", "sha256"}
        or manifest["version"] != 1
        or not isinstance(manifest["enabled"], bool)
        or (require_enabled and manifest["enabled"] is not True)
    ):
        fail("RUNNER_NOT_READY")
    if manifest["repository"] != environment.get("GITHUB_REPOSITORY") or (
        manifest["runner_name"] != environment.get("RUNNER_NAME")
        or (
            manifest["runner_name"] == "addons"
            and manifest["repository"] != "r59q/hub-rearranger"
        )
        or manifest["cli_version"] != CLI_VERSION
    ):
        fail("RUNNER_NOT_READY")
    if set(manifest["sha256"]) != REQUIRED_FILES:
        fail("RUNNER_NOT_READY")
    for name, digest in manifest["sha256"].items():
        path = trusted(RUNTIME / name)
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            fail("RUNNER_NOT_READY")
    trusted(RUNTIME / "tools/bin/python")
    if (
        WORK.is_symlink()
        or WORK.stat().st_uid != os.getuid()
        or (WORK.stat().st_mode & 0o077)
    ):
        fail("RUNNER_NOT_READY")
    for parent in WORK.parents:
        trusted(parent)
        if parent != Path.home() and (
            (parent / ".codex").exists() or (parent / "AGENTS.md").exists()
        ):
            fail("RUNNER_NOT_READY")
    # Keep existing authentication in place. User config/rules and all ambient
    # extension/tool discovery are disabled explicitly at the invocation boundary.
    if AUTH.is_symlink():
        fail("RUNNER_NOT_READY")
    if (AUTH / "AGENTS.md").exists() or (AUTH / "instructions.md").exists():
        fail("RUNNER_NOT_READY")
    info = (AUTH / "auth.json").lstat()
    if (
        not stat.S_ISREG(info.st_mode)
        or info.st_uid != os.getuid()
        or info.st_mode & 0o077
    ):
        fail("RUNNER_NOT_READY")
    return manifest


def environment(workspace, scratch, auth=False):
    result = {
        "HOME": str(scratch / "home"),
        "TMPDIR": str(scratch / "tmp"),
        "PATH": "/usr/bin:/bin",
        "LANG": "C.UTF-8",
        "CODEX_HOME": str(AUTH if auth else scratch / "codex"),
    }
    # Never inherit GH_TOKEN, GITHUB_TOKEN, Actions artifact credentials,
    # SSH agent sockets, provider overrides, proxy settings, or shell startup.
    for key in ("HOME", "TMPDIR"):
        Path(result[key]).mkdir(mode=0o700, exist_ok=True)
    if not auth:
        Path(result["CODEX_HOME"]).mkdir(mode=0o700, exist_ok=True)
    return result


def policy_arguments(workspace, scratch):
    filesystem = {
        ":root": "deny",
        ":minimal": "read",
        "/proc": "deny",
        "/sys": "deny",
        "/run": "deny",
        str(workspace): "write",
        str(scratch / "home"): "write",
        str(scratch / "tmp"): "write",
        str(AUTH): "deny",
        # Codex's sandbox reexecutes its native binary inside the namespace.
        # Expose only that reviewed executable, never the runtime's data/config.
        str((RUNTIME / "codex").resolve()): "read",
        str((RUNTIME / "codex-code-mode-host").resolve()): "read",
    }
    # Only concrete paths: no platform-dependent glob expansion. Repository
    # config/credential paths are removed before execution and checked afterward.
    options = {
        "default_permissions": "aw012",
        "permissions.aw012.filesystem": filesystem,
        "permissions.aw012.network.enabled": False,
        "approval_policy": "never",
        "shell_environment_policy.inherit": "none",
        "shell_environment_policy.set": {
            "HOME": str(scratch / "home"),
            "TMPDIR": str(scratch / "tmp"),
            "PATH": "/usr/bin:/bin",
            "LANG": "C.UTF-8",
        },
        "shell_environment_policy.experimental_use_profile": False,
        "allow_login_shell": False,
        "web_search": "disabled",
        "features.shell_snapshot": False,
        "project_doc_max_bytes": 0,
        "features.multi_agent_v2": False,
        "features.apps": False,
        "features.plugins": False,
        "features.remote_plugin": False,
        "features.hooks": False,
        "features.browser_use": False,
        "features.computer_use": False,
        "features.image_generation": False,
        "features.view_image": False,
        "features.artifact": False,
        "features.memories": False,
        "features.workspace_dependencies": False,
        "features.skill_search": False,
        "features.skill_mcp_dependency_install": False,
        "features.skip_host_skill_discovery": True,
        "features.daemon_auto_start": False,
    }
    arguments = []
    for key, value in options.items():
        # Inline TOML tables require '=', not JSON's ':'.
        arguments.extend(["-c", key + "=" + toml(value)])
    return arguments


def toml(value):
    if isinstance(value, dict):
        return (
            "{"
            + ",".join(json.dumps(k) + "=" + toml(v) for k, v in value.items())
            + "}"
        )
    return json.dumps(value)


def codex_arguments(workspace, scratch):
    return [
        "exec",
        "--ignore-user-config",
        "--ignore-rules",
        "--ephemeral",
        "--skip-git-repo-check",
        "--color",
        "never",
        "--model",
        MODEL,
        "-c",
        'model_reasoning_effort="high"',
        "-c",
        'forced_login_method="chatgpt"',
        "-c",
        'cli_auth_credentials_store="file"',
        *policy_arguments(workspace, scratch),
        "-C",
        str(workspace),
        "-",
    ]
