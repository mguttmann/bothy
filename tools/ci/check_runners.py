# SPDX-License-Identifier: Apache-2.0
"""Policy check for GitHub Actions workflows (docs/quality/ci-budget.md, ADR 0039).

One set of workflow files serves the private and the public phase of the repository.
A job either picks its runner through the private/public switch

    ${{ github.event.repository.private && fromJSON('["self-hosted","bothy-ci",...]')
        || '<github-hosted label>' }}

or it runs only in the public phase (job `if:` starts with PUBLIC_GUARD) on a
GitHub-hosted runner, or it belongs to a release workflow (RELEASE_WORKFLOWS) that
only runs for release tags. Every finding names a rule:

  invalid-yaml                  file does not parse or has no jobs
  missing-permissions           no top-level `permissions`
  broad-permissions             `permissions: write-all`
  pull-request-target           the pull_request_target trigger
  push-on-branch                a push trigger that is not limited to tags
  release-trigger               a release workflow with triggers other than tag push
                                (and workflow_dispatch where allowed)
  schedule-without-public-guard a scheduled workflow with a job lacking PUBLIC_GUARD
  unpinned-action               `uses:` not pinned to a full commit SHA (or image digest)
  missing-timeout               a job without timeout-minutes (reusable calls excepted)
  runner-without-switch         a runner chosen outside the rules above
  signing-outside-release       a signing step (cosign sign, sign-blob, attest; the `sign`
                                command of tools/ci/release_chain.py; actions/attest) in a job
                                whose environment is not `release`, the only one that holds
                                the signing key (ADR 0062 "Confirmation")

A public-only or release job may also name its runner as `${{ matrix.<key> }}` when every
value of that matrix key is a GitHub-hosted label (native builds per architecture).
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

# Release-class workflows: GitHub-hosted runners allowed, triggers limited.
RELEASE_WORKFLOWS: dict[str, frozenset[str]] = {
    "release.yml": frozenset({"push"}),
    "portability.yml": frozenset({"push", "workflow_dispatch"}),
}
PUBLIC_GUARD = (
    "github.event.repository.visibility == 'public' || "
    "(github.event_name == 'schedule' && vars.BOTHY_VISIBILITY == 'public')"
)
_HOSTED = r"(?:ubuntu|macos|windows)-[0-9a-z.]+(?:-arm)?"
_SWITCH = re.compile(
    r"^\$\{\{\s*github\.event\.repository\.private\s*&&\s*"
    r"fromJSON\('\[\s*\"self-hosted\"(?:\s*,\s*\"[A-Za-z0-9._-]+\")*\s*\]'\)\s*"
    r"\|\|\s*'(?:ubuntu|macos)-[0-9a-z.]+(?:-arm)?'\s*\}\}$"
)
_HOSTED_LABEL = re.compile(rf"^{_HOSTED}$")
_MATRIX_RUNNER = re.compile(r"^\$\{\{\s*matrix\.([A-Za-z0-9_-]+)\s*\}\}$")
_PINNED = re.compile(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[^@\s]+)?@[0-9a-f]{40}$")
_DOCKER_PINNED = re.compile(r"^docker://[^@\s]+@sha256:[0-9a-f]{64}$")
# Steps that sign: cosign signing commands, the signing command of the release chain and the
# Sigstore-backed attestation actions. Verification (`cosign verify`) is not signing.
_SIGNING_RUN = re.compile(
    r"\bcosign\s+(?:sign|sign-blob|attest|attest-blob)\b|\brelease_chain\.py\s+sign\b"
)
_SIGNING_USES = re.compile(r"^actions/attest(?:-[a-z-]+)?@")
SIGNING_ENVIRONMENT = "release"


@dataclass(frozen=True)
class Finding:
    file: str
    where: str
    rule: str
    detail: str


class _Loader(yaml.SafeLoader):
    """YAML 1.2 style booleans: `on:` stays the string 'on' instead of True."""


_Loader.yaml_implicit_resolvers = {
    key: [(tag, rx) for tag, rx in resolvers if tag != "tag:yaml.org,2002:bool"]
    for key, resolvers in yaml.SafeLoader.yaml_implicit_resolvers.items()
}
_Loader.add_implicit_resolver(
    "tag:yaml.org,2002:bool",
    re.compile(r"^(?:true|True|TRUE|false|False|FALSE)$"),
    list("tTfF"),
)


def load_yaml(text: str) -> Any:
    return yaml.load(text, Loader=_Loader)  # noqa: S506  # nosec B506


def _norm(expr: str) -> str:
    expr = expr.strip()
    if expr.startswith("${{") and expr.endswith("}}"):
        expr = expr[3:-2]
    return " ".join(expr.split())


def has_public_guard(condition: Any) -> bool:
    if not isinstance(condition, str):
        return False
    cond = _norm(condition)
    guard = _norm(PUBLIC_GUARD)
    return cond == guard or cond.startswith(f"({guard}) && ")


def _triggers(doc: dict[str, Any]) -> dict[str, Any]:
    on = doc.get("on")
    if isinstance(on, str):
        return {on: None}
    if isinstance(on, list):
        return {str(t): None for t in on}
    if isinstance(on, dict):
        return {str(k): v for k, v in on.items()}
    return {}


def _runs_on_text(runs_on: Any) -> str:
    if isinstance(runs_on, dict):
        runs_on = runs_on.get("labels", runs_on.get("group", ""))
    if isinstance(runs_on, list):
        return "[" + ",".join(str(x) for x in runs_on) + "]"
    return str(runs_on)


def _check_uses(file: str, where: str, uses: Any, out: list[Finding]) -> None:
    if not isinstance(uses, str):
        return
    if uses.startswith(("./", "$/")):  # same repository, same commit
        return
    if uses.startswith("docker://"):
        if not _DOCKER_PINNED.match(uses):
            out.append(Finding(file, where, "unpinned-action", uses))
        return
    if not _PINNED.match(uses):
        out.append(Finding(file, where, "unpinned-action", uses))


def _check_triggers(file: str, triggers: dict[str, Any], out: list[Finding]) -> None:
    if "pull_request_target" in triggers:
        out.append(Finding(file, "on", "pull-request-target", "use pull_request instead"))
    if "push" in triggers:
        push = triggers["push"] or {}
        tag_only = isinstance(push, dict) and (
            ("tags" in push or "tags-ignore" in push)
            and "branches" not in push
            and "branches-ignore" not in push
        )
        if not tag_only:
            out.append(Finding(file, "on.push", "push-on-branch", "push must be limited to tags"))
    allowed = RELEASE_WORKFLOWS.get(file)
    if allowed is not None:
        extra = sorted(set(triggers) - allowed)
        if extra:
            out.append(Finding(file, "on", "release-trigger", ", ".join(extra)))


def _check_job(
    file: str, name: str, job: dict[str, Any], scheduled: bool, out: list[Finding]
) -> None:
    where = f"job '{name}'"
    if "uses" in job:  # reusable workflow call: no runner, no timeout of its own
        _check_uses(file, where, job["uses"], out)
        return
    for i, step in enumerate(job.get("steps") or []):
        if isinstance(step, dict):
            _check_uses(file, f"{where} step {i + 1}", step.get("uses"), out)
            if _is_signing(step) and _environment(job) != SIGNING_ENVIRONMENT:
                out.append(
                    Finding(
                        file,
                        f"{where} step {i + 1}",
                        "signing-outside-release",
                        "signing steps belong in a job with environment: release",
                    )
                )
    if "timeout-minutes" not in job:
        out.append(Finding(file, where, "missing-timeout", "set timeout-minutes"))
    guarded = has_public_guard(job.get("if"))
    if scheduled and not guarded:
        out.append(
            Finding(file, where, "schedule-without-public-guard", "scheduled jobs are public-only")
        )
    runs_on = _runs_on_text(job.get("runs-on", ""))
    if _SWITCH.match(runs_on):
        return
    hosted = _HOSTED_LABEL.match(runs_on) is not None or _hosted_matrix(job, runs_on)
    if hosted and (guarded or file in RELEASE_WORKFLOWS):
        return
    out.append(Finding(file, where, "runner-without-switch", runs_on))


def _is_signing(step: dict[str, Any]) -> bool:
    """Return whether a step signs (see _SIGNING_RUN and _SIGNING_USES)."""
    run, uses = step.get("run"), step.get("uses")
    return (isinstance(run, str) and _SIGNING_RUN.search(run) is not None) or (
        isinstance(uses, str) and _SIGNING_USES.match(uses) is not None
    )


def _environment(job: dict[str, Any]) -> str:
    """Return the environment name of a job, without case and edge blanks ("" without one).

    GitHub compares environment names without case, so `Release` holds the same secrets.
    """
    env = job.get("environment")
    if isinstance(env, dict):
        env = env.get("name")
    return env.strip().lower() if isinstance(env, str) else ""


def _hosted_matrix(job: dict[str, Any], runs_on: str) -> bool:
    """Return whether `runs_on` is `${{ matrix.<key> }}` over GitHub-hosted labels only.

    A job that builds natively per architecture (one runner per platform, B12) names its runner
    through the matrix; it counts as a hosted runner when every value of that matrix key is a
    hosted label, so the same rules as for a written-out label apply.
    """
    m = _MATRIX_RUNNER.match(runs_on)
    if m is None:
        return False
    strategy = job.get("strategy")
    matrix = strategy.get("matrix") if isinstance(strategy, dict) else None
    values = matrix.get(m.group(1)) if isinstance(matrix, dict) else None
    return (
        isinstance(values, list)
        and bool(values)
        and all(isinstance(v, str) and _HOSTED_LABEL.match(v) for v in values)
    )


def check_workflow(file: str, text: str) -> list[Finding]:
    try:
        doc = load_yaml(text)
    except yaml.YAMLError as exc:
        return [Finding(file, "file", "invalid-yaml", str(exc).splitlines()[0])]
    if not isinstance(doc, dict) or not isinstance(doc.get("jobs"), dict):
        return [Finding(file, "file", "invalid-yaml", "no jobs mapping")]
    out: list[Finding] = []
    if "permissions" not in doc:
        out.append(Finding(file, "permissions", "missing-permissions", "set contents: read"))
    elif doc["permissions"] == "write-all":
        out.append(Finding(file, "permissions", "broad-permissions", "write-all"))
    triggers = _triggers(doc)
    _check_triggers(file, triggers, out)
    for name, job in doc["jobs"].items():
        if isinstance(job, dict):
            _check_job(file, str(name), job, "schedule" in triggers, out)
    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Check workflow runner and trigger policy.")
    parser.add_argument("directory", type=Path, nargs="?", default=Path(".github/workflows"))
    args = parser.parse_args(argv)
    files = sorted([*args.directory.glob("*.yml"), *args.directory.glob("*.yaml")])
    if not files:
        print(f"check_runners: no workflow files in {args.directory}", file=sys.stderr)
        return 2
    findings = [
        f for path in files for f in check_workflow(path.name, path.read_text(encoding="utf-8"))
    ]
    for f in findings:
        print(f"{f.file}: {f.where}: {f.rule} ({f.detail})")
    if findings:
        return 1
    print(f"check_runners: {len(files)} workflow file(s) comply")
    return 0


if __name__ == "__main__":
    sys.exit(main())
