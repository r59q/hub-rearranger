"""Bounded private process output, cancellation, and effective CLI policy."""

import os
import re
import signal
import subprocess
import tempfile
import time

from execution_config import EFFORT, MODEL, ExecutionFailure, fail

MAX_OUTPUT = 16 * 1024 * 1024


def invoke(arguments, directory, environment, prompt=b"", timeout=120):
    # Files live outside the workload's readable filesystem. Never return a raw
    # exception or subprocess output to GitHub. Kill descendants on all exits.
    with tempfile.TemporaryFile() as stdin, tempfile.TemporaryFile() as stdout:
        with tempfile.TemporaryFile() as stderr:
            stdin.write(prompt)
            stdin.seek(0)
            process = subprocess.Popen(
                [str(item) for item in arguments],
                cwd=directory,
                env=environment,
                stdin=stdin,
                stdout=stdout,
                stderr=stderr,
                start_new_session=True,
            )
            deadline = time.monotonic() + timeout
            try:
                while process.poll() is None:
                    if time.monotonic() >= deadline:
                        fail("RUNTIME_TIMEOUT")
                    if (
                        max(
                            os.fstat(stdout.fileno()).st_size,
                            os.fstat(stderr.fileno()).st_size,
                        )
                        > MAX_OUTPUT
                    ):
                        fail("OUTPUT_LIMIT_EXCEEDED")
                    time.sleep(0.05)
            finally:
                # Descendants can outlive their parent even after normal exit.
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
            stdout.seek(0)
            stderr.seek(0)
            output, errors = stdout.read(MAX_OUTPUT + 1), stderr.read(MAX_OUTPUT + 1)
            if len(output) > MAX_OUTPUT or len(errors) > MAX_OUTPUT:
                fail("OUTPUT_LIMIT_EXCEEDED")
            return subprocess.CompletedProcess(
                arguments, process.returncode, output, errors
            )


def effective_policy(result):
    if result.returncode:
        fail("RUNTIME_FAILED")
    models = re.findall(rb"^model: +([^\r\n]+)$", result.stderr, re.MULTILINE)
    efforts = re.findall(
        rb"^reasoning effort: +([^\r\n]+)$", result.stderr, re.MULTILINE
    )
    if (
        models != [MODEL.encode()]
        or efforts != [EFFORT.encode()]
        or re.search(
            rb"(?im)^(?:warning\b|model (?:rerouted|changed)\b)", result.stderr
        )
    ):
        fail("MODEL_POLICY_UNSUPPORTED")


def cancelled(_number, _frame):
    raise ExecutionFailure("CANCELLED")
