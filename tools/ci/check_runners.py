# SPDX-License-Identifier: Apache-2.0
"""Policy check for GitHub Actions workflows (ADR 0039).

Every job runs on a GitHub-hosted runner with a fixed label such as `ubuntu-24.04`,
`ubuntu-24.04-arm`, `macos-15` or `windows-2025`: the repository is public from its first push,
and the standard GitHub-hosted runners are free there. A job either names such a label or
`${{ matrix.<key> }}` with a static matrix whose values are all such labels. Release workflows
(RELEASE_WORKFLOWS) only run for release tags (release.yml on a tag push, portability.yml when
release.yml calls it or by hand with a product tag). Jobs with a `release*` environment receive
the signing secrets. GitHub compares environment names without case, so the check does too.
Every finding names a rule:

  invalid-yaml                  file does not parse or has no jobs
  missing-permissions           no top-level `permissions`
  broad-permissions             `permissions: write-all`, on top level or on a job
  actions-write                 `actions: write`, on top level or on a job, in any workflow:
                                with it the GITHUB_TOKEN can start workflow_dispatch runs,
                                for example of portability.yml, outside the one run per tag
                                of its release (D-W-111)
  pull-request-target           the pull_request_target trigger
  push-on-branch                a push trigger that is not limited to tags (only the
                                post-merge workflows and MAIN_PUSH_WORKFLOWS may name
                                exactly `main`, only build-verify.yml exactly `build/**`)
  release-trigger               a release, post-merge or build verification workflow with
                                other triggers than its own (release.yml: tag push;
                                portability.yml: the call from release.yml and
                                workflow_dispatch; build-verify.yml: push)
  schedule-without-public-guard a scheduled workflow with a job lacking PUBLIC_GUARD
  unpinned-action               `uses:` not pinned to a full commit SHA (or image digest)
  missing-timeout               a job without timeout-minutes (reusable calls excepted)
  runner-not-github-hosted      a runner other than a fixed GitHub-hosted label: a list, a
                                group, an expression other than a matrix value, a `-latest`
                                or larger runner label, or a label of a runner outside GitHub
  reusable-release-caller       a job calls a release workflow outside release.yml, or
                                release.yml calls one twice or from a job with a matrix
                                (a release workflow runs once per tag, from its release)
  release-untrusted-output      a `release*` environment job uses `needs` in an expression
                                outside the `if:` of the job or of one of its steps other than
                                `needs.<job>.result` or `needs.<job>.outputs.<name>` of a job
                                with such an environment (allow list: filters, toJSON, index
                                access all count; an `if` key below env or with is a value,
                                keys count too); the outputs of jobs without it may only steer
                                control flow, never reach a shell or an action
  release-artifact-by-id        a `release*` environment job downloads artifacts by name or
                                pattern instead of `artifact-ids`; any job of the run can
                                upload an artifact with a matching name
  environment-not-literal       an environment name with an expression `${{ }}`; a computed
                                name could pick a `release*` environment that no rule sees
  signing-outside-release       a signing step (cosign sign, sign-blob, attest; the `sign`
                                command of tools/ci/release_chain.py; actions/attest) in a job
                                whose environment is not `release`, the only one that holds
                                the signing key (ADR 0062 "Confirmation")
"""

from __future__ import annotations

import argparse
import itertools
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

# Release workflows: triggers limited. release.yml calls portability.yml after its publish
# job, because install.sh and the bundle exist under the tag only from then on
# (docs/spec/16-operations.md, 11.1).
RELEASE_WORKFLOWS: dict[str, frozenset[str]] = {
    "release.yml": frozenset({"push"}),
    "portability.yml": frozenset({"workflow_call", "workflow_dispatch"}),
}
# The only workflow that may call a release workflow, once per callee and without a matrix,
# so that a release workflow runs once per tag, for the release it belongs to (C4-04).
RELEASE_CALLER = "release.yml"
_CALLED_WORKFLOW = re.compile(r"(?:^|/)\.github/workflows/([^/@\s]+)(?:@|$)")
# Post-merge workflows: a push to `main` only (deny-list scan of the merged commit).
POST_MERGE_WORKFLOWS: dict[str, frozenset[str]] = {
    "denylist-main.yml": frozenset({"push", "workflow_dispatch"}),
}
# Build verification: the full gate set on hosted runners for a push to a `build/**` branch,
# nothing else (docs/quality/push-discipline.md, "Build branches"). It is no required check.
BUILD_VERIFY_WORKFLOWS: dict[str, frozenset[str]] = {"build-verify.yml": frozenset({"push"})}
BUILD_VERIFY_PUSH = {"branches": ["build/**"]}
# Workflows that also run on every push to main next to their other triggers: ci.yml, so that
# SonarQube keeps an analysis of main (docs/quality/gates.md).
MAIN_PUSH_WORKFLOWS = frozenset({"ci.yml"})
RELEASE_ENVIRONMENT_PREFIX = "release"
PUBLIC_GUARD = (
    "github.event.repository.visibility == 'public' || "
    "(github.event_name == 'schedule' && vars.BOTHY_VISIBILITY == 'public')"
)
# A fixed label of a standard GitHub-hosted runner: operating system, image version, optional
# arm64. Not `-latest` (it moves to a new image without a change here), not larger runners.
_HOSTED_LABEL = re.compile(r"^(?:ubuntu|macos|windows)-[0-9]+(?:\.[0-9]+)?(?:-arm)?$")
_PINNED = re.compile(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[^@\s]+)?@[0-9a-f]{40}$")
_DOCKER_PINNED = re.compile(r"^docker://[^@\s]+@sha256:[0-9a-f]{64}$")
# Steps that sign: cosign signing commands, the signing command of the release chain and the
# Sigstore-backed attestation actions. Verification (`cosign verify`) is not signing.
_SIGNING_RUN = re.compile(
    r"\bcosign\s+(?:sign|sign-blob|attest|attest-blob)\b|\brelease_chain\.py\s+sign\b"
)
_SIGNING_USES = re.compile(r"^actions/attest(?:-[a-z-]+)?@")
SIGNING_ENVIRONMENT = "release"
_MATRIX_REF = re.compile(r"^\$\{\{\s*matrix\.([A-Za-z0-9_-]+)\s*\}\}$")
# Release jobs: every `needs` inside `${{ }}` outside `if:` is a finding unless it is
# exactly `needs.<job>.result` or `needs.<release job>.outputs.<name>` (allow list, K3-04).
# Expressions ignore case; an expression ends at the first `}}` outside a string literal
# ('...', with '' as an escaped quote), as GitHub reads it; an unclosed `${{` counts up to
# the end of the text. `needs` inside a string literal still counts (conservative).
_NEEDS_WORD = re.compile(r"(?<![A-Za-z0-9_.-])needs(?![A-Za-z0-9_-])", re.IGNORECASE)
_NEEDS_ALLOWED = re.compile(
    r"needs\.([A-Za-z0-9_-]+)\.(?:(result)|outputs\.[A-Za-z0-9_-]+)(?![A-Za-z0-9_.\[(-])"
)
_DOWNLOAD_ARTIFACT = re.compile(r"^actions/download-artifact@")


@dataclass(frozen=True)
class Finding:
    """One policy violation: workflow file, place in the file, rule name and detail."""

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
    """Parse workflow YAML with YAML 1.2 booleans so that the key `on` stays a string."""
    return yaml.load(text, Loader=_Loader)  # noqa: S506  # nosec B506


def _norm(expr: str) -> str:
    expr = expr.strip()
    if expr.startswith("${{") and expr.endswith("}}"):
        expr = expr[3:-2]
    return " ".join(expr.split())


def has_public_guard(condition: Any) -> bool:
    """Return True if a job condition is PUBLIC_GUARD itself or starts with it."""
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
        main_only = (file in POST_MERGE_WORKFLOWS or file in MAIN_PUSH_WORKFLOWS) and push == {
            "branches": ["main"]
        }
        build_only = file in BUILD_VERIFY_WORKFLOWS and push == BUILD_VERIFY_PUSH
        if not tag_only and not main_only and not build_only:
            out.append(Finding(file, "on.push", "push-on-branch", "push must be limited to tags"))
    allowed = (
        RELEASE_WORKFLOWS.get(file)
        or POST_MERGE_WORKFLOWS.get(file)
        or BUILD_VERIFY_WORKFLOWS.get(file)
    )
    if allowed is not None:
        extra = sorted(set(triggers) - allowed)
        if extra:
            out.append(Finding(file, "on", "release-trigger", ", ".join(extra)))


def matrix_combinations(job: dict[str, Any]) -> list[dict[str, Any]] | None:
    """Return the combinations of a job's static `strategy.matrix` ([{}] without a matrix).

    Follows the documented expansion: the product of the list-valued keys, minus every
    combination that matches an `exclude` entry, then each `include` entry is added to every
    combination whose original values it does not overwrite, or becomes a combination of its
    own. Returns None when the matrix (or one of its keys) is computed at run time.
    """
    strategy = job.get("strategy")
    if strategy is None:
        return [{}]
    if not isinstance(strategy, dict):
        return None
    matrix = strategy.get("matrix")
    if matrix is None:
        return [{}]
    if not isinstance(matrix, dict):
        return None
    include = matrix.get("include", [])
    exclude = matrix.get("exclude", [])
    axes = {k: v for k, v in matrix.items() if k not in ("include", "exclude")}
    if not isinstance(include, list) or not isinstance(exclude, list):
        return None
    if any(not isinstance(v, list) for v in axes.values()):
        return None
    if any(not isinstance(e, dict) for e in [*include, *exclude]):
        return None
    combos = [dict(zip(axes, values, strict=True)) for values in itertools.product(*axes.values())]
    if not axes:
        combos = []
    combos = [
        c for c in combos if not any(all(c.get(k) == v for k, v in e.items()) for e in exclude)
    ]
    originals = [dict(c) for c in combos]
    extra: list[dict[str, Any]] = []
    for entry in include:
        matched = False
        for original, combo in zip(originals, combos, strict=True):
            if all(original.get(k, v) == v for k, v in entry.items()):
                combo.update(entry)
                matched = True
        if not matched:
            extra.append(dict(entry))
    return combos + extra


def runner_labels(job: dict[str, Any]) -> list[str] | None:
    """Return the runner text of every matrix combination; None if it cannot be known."""
    runs_on = _runs_on_text(job.get("runs-on", ""))
    combos = matrix_combinations(job)
    ref = _MATRIX_REF.match(runs_on)
    if ref is None:
        return None if combos is None else [runs_on] * len(combos)
    if combos is None:
        return None
    return [_runs_on_text(c.get(ref.group(1), runs_on)) for c in combos]


def _is_signing(step: dict[str, Any]) -> bool:
    """Return whether a step signs (see _SIGNING_RUN and _SIGNING_USES)."""
    run, uses = step.get("run"), step.get("uses")
    return (isinstance(run, str) and _SIGNING_RUN.search(run) is not None) or (
        isinstance(uses, str) and _SIGNING_USES.match(uses) is not None
    )


def _check_job(
    file: str, name: str, job: dict[str, Any], scheduled: bool, out: list[Finding]
) -> None:
    where = f"job '{name}'"
    if "permissions" in job:
        _check_permissions(file, where, job["permissions"], out)
    _check_environment_literal(file, where, job, out)
    if "uses" in job:  # reusable workflow call: no runner, no timeout of its own
        _check_uses(file, where, job["uses"], out)
        return
    for i, step in enumerate(job.get("steps") or []):
        if isinstance(step, dict):
            _check_uses(file, f"{where} step {i + 1}", step.get("uses"), out)
            if _is_signing(step) and _environment(job).strip().casefold() != SIGNING_ENVIRONMENT:
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
    if scheduled and not has_public_guard(job.get("if")):
        out.append(
            Finding(file, where, "schedule-without-public-guard", "scheduled jobs are public-only")
        )
    runs_on = _runs_on_text(job.get("runs-on", ""))
    labels = runner_labels(job)
    if labels is None:
        labels = [runs_on]
    for label in dict.fromkeys(labels):
        if not _HOSTED_LABEL.match(label):
            out.append(Finding(file, where, "runner-not-github-hosted", label))


def _strings(node: Any) -> list[str]:
    """Return every string in a YAML node, mapping keys included (conservative)."""
    if isinstance(node, str):
        return [node]
    if isinstance(node, list):
        return [s for item in node for s in _strings(item)]
    if isinstance(node, dict):
        return [s for k, v in node.items() for s in (*_strings(k), *_strings(v))]
    return []


def _strings_outside_if(node: dict[Any, Any]) -> list[str]:
    return [s for k, v in node.items() if k != "if" for s in (*_strings(k), *_strings(v))]


def _expressions_outside_if(job: dict[str, Any]) -> list[str]:
    """Return every string of a job except its own `if:` and the `if:` of its steps.

    Only these two are conditions. A key `if` anywhere else (`env.if`, `with.if`) is an
    ordinary value that reaches a shell or an action, so it is checked (C4-03).
    """
    strings: list[str] = []
    for key, value in job.items():
        if key == "steps" and isinstance(value, list):
            strings += _strings(key)
            for step in value:
                strings += _strings_outside_if(step) if isinstance(step, dict) else _strings(step)
        elif key != "if":
            strings += [*_strings(key), *_strings(value)]
    return strings


def _expressions(text: str) -> list[str]:
    """Return the body of every `${{ ... }}` in `text`, skipping `}}` inside '...' literals."""
    bodies: list[str] = []
    start = text.find("${{")
    while start != -1:
        i = start + 3
        in_string = False
        end = len(text)
        while i < len(text):
            if in_string:
                if text.startswith("''", i):
                    i += 2
                    continue
                if text[i] == "'":
                    in_string = False
            elif text[i] == "'":
                in_string = True
            elif text.startswith("}}", i):
                end = i
                break
            i += 1
        bodies.append(text[start + 3 : end])
        start = text.find("${{", end + 2) if end < len(text) else -1
    return bodies


def _untrusted_needs(text: str, trusted: set[str]) -> list[str]:
    """Return every `needs` use in the expressions of `text` outside the allow list."""
    found: list[str] = []
    for body in _expressions(text):
        for word in _NEEDS_WORD.finditer(body):
            allowed = _NEEDS_ALLOWED.match(body, word.start())
            if allowed and (allowed.group(2) or allowed.group(1) in trusted):
                continue
            found.append(" ".join(body[word.start() :].split())[:60])
    return found


def _check_release_inputs(file: str, jobs: dict[str, Any], out: list[Finding]) -> None:
    """Keep values of jobs without a release environment out of signing jobs (T-SC-05)."""
    trusted = {str(n) for n, j in jobs.items() if isinstance(j, dict) and is_release_environment(j)}
    for name in sorted(trusted):
        job = jobs[name]
        where = f"job '{name}'"
        for text in _expressions_outside_if(job):
            for use in _untrusted_needs(text, trusted):
                out.append(
                    Finding(
                        file,
                        where,
                        "release-untrusted-output",
                        f"{use}: use it only in if:, resolve again on this runner",
                    )
                )
        for i, step in enumerate(job.get("steps") or []):
            if not isinstance(step, dict) or not _DOWNLOAD_ARTIFACT.match(
                str(step.get("uses", ""))
            ):
                continue
            inputs = step.get("with") or {}
            if "artifact-ids" not in inputs or "name" in inputs or "pattern" in inputs:
                out.append(
                    Finding(
                        file, f"{where} step {i + 1}", "release-artifact-by-id", "use artifact-ids"
                    )
                )


def _check_release_callers(file: str, jobs: dict[str, Any], out: list[Finding]) -> None:
    """A release workflow runs once per tag: called by one job of RELEASE_CALLER only."""
    called: set[str] = set()
    for name, job in jobs.items():
        if not isinstance(job, dict) or not isinstance(job.get("uses"), str):
            continue
        match = _CALLED_WORKFLOW.search(job["uses"])
        if match is None or match.group(1) not in RELEASE_WORKFLOWS:
            continue
        target = match.group(1)
        where = f"job '{name}'"
        if file != RELEASE_CALLER:
            detail = f"only {RELEASE_CALLER} may call {target}"
            out.append(Finding(file, where, "reusable-release-caller", detail))
        elif target in called or "strategy" in job:
            detail = f"call {target} from one job without a matrix"
            out.append(Finding(file, where, "reusable-release-caller", detail))
        called.add(target)


def _environment(job: dict[str, Any]) -> str:
    env = job.get("environment")
    if isinstance(env, dict):
        env = env.get("name")
    return str(env) if env is not None else ""


def is_release_environment(job: dict[str, Any]) -> bool:
    """Return True if the job references a `release*` environment in any spelling.

    GitHub treats environment names without case ("Environment names are not case
    sensitive"), so `Release` and `RELEASE-apple` hold the secrets of `release*` too.
    """
    return _environment(job).strip().casefold().startswith(RELEASE_ENVIRONMENT_PREFIX)


def _check_environment_literal(
    file: str, where: str, job: dict[str, Any], out: list[Finding]
) -> None:
    if "${{" in _environment(job):
        out.append(
            Finding(file, where, "environment-not-literal", "name the environment literally")
        )


def _check_permissions(file: str, where: str, permissions: Any, out: list[Finding]) -> None:
    """No `write-all` and no `actions: write`: no workflow_dispatch from a workflow.

    A repository_dispatch needs only `contents: write`; it starts only workflows that name
    that trigger, which rule release-trigger forbids for the release workflows.
    """
    if permissions == "write-all":
        out.append(Finding(file, where, "broad-permissions", "write-all"))
    elif isinstance(permissions, dict) and str(permissions.get("actions", "")) == "write":
        out.append(Finding(file, where, "actions-write", "no workflow_dispatch from a workflow"))


def check_workflow(file: str, text: str) -> list[Finding]:
    """Return every policy finding of one workflow file (`file` is its base name)."""
    try:
        doc = load_yaml(text)
    except yaml.YAMLError as exc:
        return [Finding(file, "file", "invalid-yaml", str(exc).splitlines()[0])]
    if not isinstance(doc, dict) or not isinstance(doc.get("jobs"), dict):
        return [Finding(file, "file", "invalid-yaml", "no jobs mapping")]
    out: list[Finding] = []
    if "permissions" not in doc:
        out.append(Finding(file, "permissions", "missing-permissions", "set contents: read"))
    else:
        _check_permissions(file, "permissions", doc["permissions"], out)
    triggers = _triggers(doc)
    _check_triggers(file, triggers, out)
    for name, job in doc["jobs"].items():
        if isinstance(job, dict):
            _check_job(file, str(name), job, "schedule" in triggers, out)
    _check_release_inputs(file, doc["jobs"], out)
    _check_release_callers(file, doc["jobs"], out)
    return out


def main(argv: list[str] | None = None) -> int:
    """Check every workflow in the directory; exit code 0 clean, 1 findings, 2 no files."""
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
