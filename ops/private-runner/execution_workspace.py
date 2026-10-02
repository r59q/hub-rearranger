"""Bounded checkout materialization and patch capture outside workload Git state."""

import io
import stat
import subprocess
import tarfile
from pathlib import PurePosixPath

from execution_config import fail

MAX_SOURCE = 128 * 1024 * 1024
MAX_PATCH = 8 * 1024 * 1024


def protected(path):
    parts = PurePosixPath(path).parts
    return (
        not parts
        or any(p in {".git", ".codex", ".agents"} for p in parts)
        or any(p == "AGENTS.md" or p == ".env" or p.startswith(".env.") for p in parts)
        or parts[0] == ".github"
        or parts[:2]
        in {
            ("ops", "private-runner"),
            ("ops", "agent-profiles"),
            ("ops", "agent-intake"),
        }
        or ".gitmodules" in parts
    )


def omitted(path):
    return any(
        part in {".git", ".codex", ".agents", "AGENTS.md"}
        or (part == ".env" or part.startswith(".env."))
        for part in PurePosixPath(path).parts
    )


def extract(source, workspace, instructions=False):
    total = 0
    count = 0
    root = None
    seen = set()
    rules = []
    with tarfile.open(fileobj=io.BytesIO(source), mode="r:gz") as archive:
        for member in archive:
            count += 1
            total += member.size
            parts = PurePosixPath(member.name).parts
            if count > 20000 or total > MAX_SOURCE or member.size > 16 * 1024 * 1024:
                fail("SOURCE_LIMIT_EXCEEDED")
            if (
                not parts
                or member.name.startswith("/")
                or ".." in parts
                or (any("\\" in p or any(ord(c) < 32 for c in p) for p in parts))
            ):
                fail("UNSAFE_SOURCE")
            root = root or parts[0]
            if parts[0] != root:
                fail("UNSAFE_SOURCE")
            if len(parts) == 1 and member.isdir():
                continue
            name = "/".join(parts[1:])
            if not name or name in seen or not (member.isfile() or member.isdir()):
                fail("UNSAFE_SOURCE")
            seen.add(name)
            if ".gitmodules" in parts:
                fail("UNSAFE_SOURCE")
            if omitted(name):
                if parts[-1] == "AGENTS.md" and instructions and member.isfile():
                    if (
                        member.size > 32768
                        or sum(len(r["body"]) for r in rules) > 65536
                    ):
                        fail("SOURCE_LIMIT_EXCEEDED")
                    rules.append(
                        {
                            "path": name,
                            "body": archive.extractfile(member).read().decode("utf-8"),
                        }
                    )
                continue
            destination = workspace / name
            if member.isdir():
                destination.mkdir(parents=True, exist_ok=True)
                continue
            destination.parent.mkdir(parents=True, exist_ok=True)
            with archive.extractfile(member) as stream:
                destination.write_bytes(stream.read())
            destination.chmod(0o755 if member.mode & 0o111 else 0o644)
    if root is None:
        fail("UNSAFE_SOURCE")
    return rules


def git(directory, workspace, arguments):
    # Workload code cannot modify the private index/config or inject Git hooks,
    # filters, global config, credentials, or a replacement Git executable.
    process = subprocess.run(
        [
            "/usr/bin/git",
            "--no-optional-locks",
            "-c",
            "core.hooksPath=/dev/null",
            "-c",
            "core.attributesFile=/dev/null",
            "-c",
            "core.autocrlf=false",
            "--git-dir=" + str(directory),
            *([] if arguments[0] == "init" else ["--work-tree=" + str(workspace)]),
            *arguments,
        ],
        cwd=workspace,
        env={
            "PATH": "/usr/bin:/bin",
            "LANG": "C.UTF-8",
            "HOME": str(directory.parent),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_SYSTEM": "/dev/null",
            "GIT_ATTR_NOSYSTEM": "1",
        },
        stdin=subprocess.DEVNULL,
        capture_output=True,
        check=False,
    )
    if process.returncode:
        fail("PATCH_UNAVAILABLE")
    return process.stdout


def baseline(directory, workspace):
    git(directory, workspace, ["init", "--bare", str(directory)])
    git(directory, workspace, ["add", "--all", "--force"])
    return git(directory, workspace, ["write-tree"]).decode().strip()


def patch(directory, workspace, tree):
    count, size = 0, 0
    for path in workspace.rglob("*"):
        relative = path.relative_to(workspace).as_posix()
        info = path.lstat()
        count += 1
        size += info.st_size
        if count > 20000 or size > MAX_SOURCE:
            fail("SOURCE_LIMIT_EXCEEDED")
        if (
            stat.S_ISLNK(info.st_mode)
            or not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode))
            or (stat.S_ISREG(info.st_mode) and info.st_nlink != 1)
        ):
            fail("PROTECTED_CHANGE")
        if omitted(relative) or ".gitmodules" in PurePosixPath(relative).parts:
            fail("PROTECTED_CHANGE")
    # Preserve tracked source, and honor repository ignores for newly generated
    # build/dependency output. The private baseline already tracks original files.
    git(directory, workspace, ["add", "--all"])
    changed = git(
        directory, workspace, ["diff", "--cached", "--name-only", "-z", tree]
    ).split(b"\0")
    if any(protected(name.decode("utf-8")) for name in changed if name):
        fail("PROTECTED_CHANGE")
    content = git(
        directory,
        workspace,
        ["diff", "--cached", "--binary", "--no-ext-diff", "--no-textconv", tree],
    )
    if not content:
        fail("NO_CHANGES")
    if len(content) > MAX_PATCH:
        fail("PATCH_LIMIT_EXCEEDED")
    return content
