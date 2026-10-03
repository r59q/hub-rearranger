"""Prove that AW-008 output works without Hub or the upstream repository tree."""

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).absolute().parents[3]
SERVICE = ROOT / "services/agents"


def main():
    with tempfile.TemporaryDirectory(prefix="hub-bootstrap-check-") as name:
        temporary = Path(name)
        binary = temporary / "agent-bootstrap"
        package = temporary / "repository"
        subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd/bootstrap"],
            cwd=SERVICE,
            check=True,
        )
        subprocess.run(
            [str(binary), "--source", str(ROOT), "--output", str(package)],
            stdout=subprocess.DEVNULL,
            check=True,
        )
        command = [str(binary), "--source", str(ROOT), "--target", str(package)]
        comparison = subprocess.run(command, capture_output=True, check=True)
        plan = json.loads(comparison.stdout)
        if plan["diagnostics"] or any(
            file["status"] != "unchanged" for file in plan["files"]
        ):
            raise RuntimeError("Exported bootstrap package did not converge")
        diff = subprocess.run(
            [*command, "--format", "diff"], capture_output=True, check=True
        )
        if diff.stdout:
            raise RuntimeError("Repeated generation produced a diff")

        # Reuse the pinned development environment only for this offline check;
        # the exported package itself contains no virtualenv or local state.
        python = Path(sys.executable).absolute()
        (package / "ops/private-runner/.venv").symlink_to(python.parent.parent)
        environment = dict(os.environ, INTAKE_PYTHON=str(python))
        workflow = yaml.safe_load(
            (package / ".github/workflows/agent-bootstrap-checks.yml").read_text()
        )
        checks = workflow["jobs"]["check"]["steps"][-1]["run"]
        subprocess.run(
            ["bash", "-euo", "pipefail", "-c", checks],
            cwd=package,
            env=environment,
            check=True,
        )
        print(
            f"Portable bootstrap verified: {len(plan['files'])} files; "
            "repeat diff empty."
        )


if __name__ == "__main__":
    main()
