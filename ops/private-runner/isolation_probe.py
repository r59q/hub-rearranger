"""Runs inside the same no-network, restricted-read sandbox as source commands."""

import errno
import json
import os
import socket
import sys
from pathlib import Path


def probe(paths, workspace):
    # A broken probe must fail rather than count denied execution as isolation.
    target = Path(workspace) / "isolation-canary"
    target.write_text("permitted")
    if target.read_text() != "permitted":
        return False
    target.unlink()
    for value in paths:
        try:
            with open(value, "rb") as file:
                file.read(1)
        except OSError as error:
            if error.errno not in {errno.EPERM, errno.EACCES, errno.ENOENT}:
                return False
            continue
        return False
    if any(
        name in os.environ
        for name in (
            "GH_TOKEN",
            "GITHUB_TOKEN",
            "OPENAI_API_KEY",
            "CODEX_API_KEY",
            "SSH_AUTH_SOCK",
            "NODE_OPTIONS",
            "BASH_ENV",
            "CODEX_APPS_SOCKET_PATH",
        )
    ):
        return False
    for family, address in (
        (socket.AF_INET, ("127.0.0.1", 9)),
        (socket.AF_INET, ("1.1.1.1", 443)),
    ):
        try:
            connection = socket.socket(family, socket.SOCK_STREAM)
            connection.settimeout(1)
            connection.connect(address)
        except OSError as error:
            # Offline connectivity, a refused port, or a timeout is not proof
            # of enforcement. Require an actual permission/network denial.
            if error.errno not in {errno.EPERM, errno.EACCES, errno.ENETUNREACH}:
                return False
            continue
        else:
            connection.close()
            return False
    return True


def main():
    request = json.load(sys.stdin)
    success = probe(request["denied_paths"], request["workspace"])
    print("ISOLATION_VERIFIED" if success else "ISOLATION_FAILED")
    return 0 if success else 1


if __name__ == "__main__":
    raise SystemExit(main())
