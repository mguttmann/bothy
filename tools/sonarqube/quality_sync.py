# SPDX-License-Identifier: Apache-2.0
"""Apply and check the project's SonarQube quality gate and quality profiles.

The repository holds the truth (quality/sonar/): quality-gate.json, profiles.json and
one quality-profile-<language>.xml per language with the rules activated IN ADDITION to
the parent profile. The same files are applied to the local SonarQube Community Build
and to SonarQube Cloud; `check` against both proves they enforce the same gate and the
same profile delta (docs/quality/sonarqube-local.md).

Usage (token only from the environment variable SONAR_TOKEN, never printed):
  quality_sync.py apply --url http://127.0.0.1:9000
  quality_sync.py check --url https://sonarcloud.io --organization <org> --project <key>
  quality_sync.py main-branch --url https://sonarcloud.io --organization <org> --project <key>
  quality_sync.py ce-error --url https://sonarcloud.io --organization <org>

`ce-error` prints why the server's background task of the last scan failed: the scanner only
reports "CE Task finished abnormally", the reason stays on the server (api/ce/task with the
ceTaskId of .scannerwork/report-task.txt).

`main-branch` names the project's main branch `main` (api/project_branches/rename) when it
carries another name, such as the `master` a new SonarQube Cloud project starts with; until
then the server takes an analysis of `main` for a short-lived branch and fails it. A
short-lived branch `main` those analyses left is deleted before the rename.

Languages the server does not know (Dart and Swift on the Community Build) are skipped
and listed; the Cloud check covers them.

SonarQube Cloud refuses a custom default gate (HTTP 403 on set_as_default) and a custom
default profile (HTTP 400 on set_default) on plans that do not include them. In cloud mode
the profiles are therefore never made the organization default: apply assigns each profile
and the gate to the project (--project, SONAR_PROJECT_KEY) instead. A call the plan refuses
is a reported gap, not a failure, and apply continues with the next language; check reports
a project still on a built-in profile or gate as a gap while every other difference fails.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

DEFAULT_CONFIG = Path(__file__).resolve().parents[2] / "quality" / "sonar"
DEFAULT_REPORT_TASK = Path(".scannerwork") / "report-task.txt"
# How SonarQube Cloud words a refusal by the organization's plan, for example
# "Organization '<id>' is not allowed to perform this action" (set_default, add_project) or
# "... is not allowed to modify Quality gates" (select), both seen on 2026-10-14.
_PLAN_REFUSAL = re.compile(r": Organization '[^']*' is not allowed to ")
_RULE = re.compile(r"<repositoryKey>([^<]+)</repositoryKey>\s*<key>([^<]+)</key>")
_TIMEOUT_S = 60
_MAX_ERROR_CHARS = 300
_MAX_ERROR_BODY_BYTES = 65536
GATE_NOT_DEFAULT = "gate not default: plan does not allow it"
GATE_NOT_ON_PROJECT = "gate not assigned to project: plan does not allow it"
PROFILE_NOT_ON_PROJECT = "not assigned to project: plan does not allow it"


class SonarError(Exception):
    """The server rejected a call or lacks something the configuration needs."""

    def __init__(self, message: str, status: int | None = None) -> None:
        """Keep the HTTP status of a rejected call (None if there was no HTTP answer)."""
        super().__init__(message)
        self.status = status


@dataclass(frozen=True)
class Condition:
    """One quality gate condition: metric, operator and error threshold."""

    metric: str
    op: str
    error: str


@dataclass(frozen=True)
class Config:
    """The shared gate and profile definition from `quality/sonar/`."""

    gate_name: str
    conditions: list[Condition]
    profile_name: str
    parent: str
    profiles: dict[str, Path]
    cloud_parent: str = ""

    def parent_for(self, client: Client) -> str:
        """Return the built-in parent profile on this server (SonarQube Cloud has its own)."""
        return (self.cloud_parent or self.parent) if client.is_cloud else self.parent


@dataclass
class Report:
    """Problems found, known gaps the server's plan imposes and languages it does not analyse."""

    problems: list[str] = field(default_factory=list)
    gaps: list[str] = field(default_factory=list)
    skipped_languages: list[str] = field(default_factory=list)


def load_config(directory: Path) -> Config:
    """Load gate and profiles; raises SonarError if a profile does not match profiles.json."""
    gate = json.loads((directory / "quality-gate.json").read_text(encoding="utf-8"))
    manifest = json.loads((directory / "profiles.json").read_text(encoding="utf-8"))
    conditions = [Condition(c["metric"], c["op"], str(c["error"])) for c in gate["conditions"]]
    profiles = {lang: directory / name for lang, name in sorted(manifest["languages"].items())}
    for lang, path in profiles.items():
        text = path.read_text(encoding="utf-8")
        if f"<language>{lang}</language>" not in text or (
            f"<name>{manifest['profile_name']}</name>" not in text
        ):
            raise SonarError(f"{path.name}: name or language does not match profiles.json")
    return Config(
        gate["name"],
        conditions,
        manifest["profile_name"],
        manifest["parent"],
        profiles,
        manifest.get("cloud_parent", ""),
    )


def xml_rules(text: str) -> set[str]:
    """Return the active rules `repository:key` of a profile backup XML."""
    return {f"{repo}:{key}" for repo, key in _RULE.findall(text)}


def _read_error_body(response: Any) -> bytes:
    """Read at most _MAX_ERROR_BODY_BYTES of an error response (b'' if reading fails)."""
    try:
        body: bytes = response.read(_MAX_ERROR_BODY_BYTES)
    except OSError:
        return b""
    return body


def _error_messages(body: bytes) -> str:
    """Return ': <msg>; <msg>' from a Web API error body, or '' if it carries none."""
    try:
        errors = json.loads(body)["errors"]
        text = "; ".join(str(e["msg"]) for e in errors)
    except (ValueError, KeyError, TypeError):
        return ""
    return f": {text[:_MAX_ERROR_CHARS]}" if text else ""


class Client:
    """Minimal SonarQube Web API client (token auth, optional organization for the cloud)."""

    def __init__(
        self, url: str, token: str, organization: str | None = None, project: str | None = None
    ) -> None:
        """Create a client; raises SonarError for a URL scheme other than http or https.

        `project` is the project key the cloud assigns gate and profiles to.
        """
        if urllib.parse.urlsplit(url).scheme not in ("http", "https"):
            raise SonarError(f"unsupported URL scheme in {url!r}; use http or https")
        self._url = url.rstrip("/")
        self._token = token
        self._organization = organization
        self.is_cloud = organization is not None
        self.project = project

    def _params(self, params: dict[str, str]) -> dict[str, str]:
        return {**params, "organization": self._organization} if self._organization else params

    def _send(self, req: urllib.request.Request) -> bytes:
        req.add_header("Authorization", f"Bearer {self._token}")
        try:
            # The scheme is restricted to http/https in __init__.
            with urllib.request.urlopen(req, timeout=_TIMEOUT_S) as resp:  # noqa: S310  # nosec B310
                body: bytes = resp.read()
                return body
        except urllib.error.HTTPError as exc:
            reason = _error_messages(_read_error_body(exc))
            exc.close()
            if exc.code == 404:
                raise LookupError(req.full_url.split("?")[0]) from None
            raise SonarError(
                f"{req.get_method()} {req.full_url.split('?')[0]}: HTTP {exc.code}{reason}",
                exc.code,
            ) from None

    def get(self, path: str, **params: str) -> bytes:
        """Send a GET request and return the body; raises SonarError on an HTTP error."""
        query = urllib.parse.urlencode(self._params(params))
        return self._send(urllib.request.Request(f"{self._url}/{path}?{query}"))  # noqa: S310

    def get_json(self, path: str, **params: str) -> Any:
        """Send a GET request and return the parsed JSON body."""
        return json.loads(self.get(path, **params))

    def post(self, path: str, **params: str) -> None:
        """Send a form-encoded POST request."""
        data = urllib.parse.urlencode(self._params(params)).encode()
        req = urllib.request.Request(f"{self._url}/{path}", data=data, method="POST")  # noqa: S310
        self._send(req)

    def post_file(self, path: str, field_name: str, filename: str, content: str) -> None:
        """Upload `content` as a multipart file field (profile restore)."""
        boundary = uuid.uuid4().hex
        parts = [
            f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'
            for k, v in self._params({}).items()
        ]
        parts.append(
            f'--{boundary}\r\nContent-Disposition: form-data; name="{field_name}"; '
            f'filename="{filename}"\r\nContent-Type: application/xml\r\n\r\n{content}\r\n'
        )
        body = ("".join(parts) + f"--{boundary}--\r\n").encode()
        query = urllib.parse.urlencode(self._params({}))
        url = f"{self._url}/{path}?{query}"
        req = urllib.request.Request(url, data=body, method="POST")  # noqa: S310
        req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
        self._send(req)


def _languages(client: Client) -> set[str]:
    return {lang["key"] for lang in client.get_json("api/languages/list")["languages"]}


def _gate(client: Client, name: str) -> tuple[str, list[dict[str, str]]] | None:
    """Return id and conditions of the gate `name`, or None if it does not exist."""
    try:
        gate = client.get_json("api/qualitygates/show", name=name)
    except LookupError:
        return None
    return str(gate.get("id", "")), list(gate["conditions"])


def _gate_is_default(client: Client, name: str) -> bool:
    gates = client.get_json("api/qualitygates/list")["qualitygates"]
    return any(g["name"] == name and g.get("isDefault") for g in gates)


def _gate_ref(
    client: Client, id_key: str, gate_id: str, name_key: str, name: str
) -> dict[str, str]:
    """Address a gate: SonarQube Cloud wants its id, SonarQube Server its name."""
    return {id_key: gate_id} if client.is_cloud else {name_key: name}


def _plan_refused(client: Client, exc: SonarError) -> bool:
    """Whether SonarQube Cloud refused a call because the organization's plan lacks it.

    Only the plan's wording counts (_PLAN_REFUSAL); any other refusal, such as HTTP 403
    "Insufficient privileges" for a token without the permission, still fails the run.
    """
    return (
        client.is_cloud and exc.status in (400, 403) and _PLAN_REFUSAL.search(str(exc)) is not None
    )


def _cloud_project(client: Client) -> str:
    """Return the project key the cloud mode assigns to; raises SonarError if it is missing."""
    if not client.project:
        raise SonarError("SonarQube Cloud needs the project key (--project or SONAR_PROJECT_KEY)")
    return client.project


def _profile(client: Client, language: str, name: str) -> dict[str, Any] | None:
    found = client.get_json("api/qualityprofiles/search", language=language, qualityProfile=name)
    return next((p for p in found["profiles"] if p["name"] == name), None)


def _active_rules(client: Client, language: str, name: str) -> set[str]:
    xml = client.get("api/qualityprofiles/backup", language=language, qualityProfile=name)
    return xml_rules(xml.decode("utf-8"))


def _apply_gate(client: Client, config: Config, report: Report) -> str:
    """Create or update the gate, make it the default and return its id."""
    found = _gate(client, config.gate_name)
    if found is None:
        client.post("api/qualitygates/create", name=config.gate_name)
        found = _gate(client, config.gate_name)
        if found is None:
            raise SonarError(f"gate '{config.gate_name}' was not created")
    gate_id, current = found
    wanted = {c.metric: c for c in config.conditions}
    for cond in current:
        want = wanted.get(cond["metric"])
        if want is None:
            client.post("api/qualitygates/delete_condition", id=cond["id"])
        elif (cond.get("op"), cond["error"]) != (want.op, want.error):
            client.post(
                "api/qualitygates/update_condition",
                id=cond["id"],
                metric=want.metric,
                op=want.op,
                error=want.error,
            )
    have = {c["metric"] for c in current}
    for want in config.conditions:
        if want.metric not in have:
            client.post(
                "api/qualitygates/create_condition",
                **_gate_ref(client, "gateId", gate_id, "gateName", config.gate_name),
                metric=want.metric,
                op=want.op,
                error=want.error,
            )
    try:
        client.post(
            "api/qualitygates/set_as_default",
            **_gate_ref(client, "id", gate_id, "name", config.gate_name),
        )
    except SonarError as exc:
        if not (client.is_cloud and exc.status == 403):
            raise
        report.gaps.append(GATE_NOT_DEFAULT)
    return gate_id


def apply(client: Client, config: Config) -> Report:
    """Create or update gate and profiles so that the server matches the config."""
    report = Report()
    project = _cloud_project(client) if client.is_cloud else ""
    gate_id = _apply_gate(client, config, report)

    parent = config.parent_for(client)
    available = _languages(client)
    for language, xml_path in config.profiles.items():
        if language not in available:
            report.skipped_languages.append(language)
            continue
        if _profile(client, language, parent) is None:
            raise SonarError(
                f"{language}: parent profile '{parent}' does not exist on this server; "
                "adjust quality/sonar/profiles.json after checking the built-in profiles"
            )
        if _profile(client, language, config.profile_name) is None:
            client.post("api/qualityprofiles/create", language=language, name=config.profile_name)
        client.post(
            "api/qualityprofiles/change_parent",
            language=language,
            qualityProfile=config.profile_name,
            parentQualityProfile=parent,
        )
        client.post_file(
            "api/qualityprofiles/restore",
            "backup",
            xml_path.name,
            xml_path.read_text(encoding="utf-8"),
        )
        if not client.is_cloud:
            client.post(
                "api/qualityprofiles/set_default",
                language=language,
                qualityProfile=config.profile_name,
            )
            continue
        try:
            client.post(
                "api/qualityprofiles/add_project",
                language=language,
                qualityProfile=config.profile_name,
                project=project,
            )
        except SonarError as exc:
            if not _plan_refused(client, exc):
                raise
            report.gaps.append(f"profile {language}: {PROFILE_NOT_ON_PROJECT} ({exc})")
    if client.is_cloud:
        # After the profiles, so that a refused gate cannot keep them from the project.
        try:
            client.post("api/qualitygates/select", gateId=gate_id, projectKey=project)
        except SonarError as exc:
            if not _plan_refused(client, exc):
                raise
            report.gaps.append(f"{GATE_NOT_ON_PROJECT} ({exc})")
    return report


def ensure_main_branch(client: Client, name: str) -> str | None:
    """Rename the project's main branch to `name`; return its old name, None if unchanged.

    Raises SonarError if the server refuses the rename or the project has no main branch.
    """
    project = _cloud_project(client) if client.is_cloud else client.project
    if not project:
        raise SonarError("the main branch needs the project key (--project or SONAR_PROJECT_KEY)")
    branches = client.get_json("api/project_branches/list", project=project)["branches"]
    current = next((b["name"] for b in branches if b.get("isMain")), None)
    if current is None:
        raise SonarError(f"project '{project}' has no main branch")
    if current == name:
        return None
    # While the main branch had another name, the analyses of `name` left a short-lived branch
    # of that name, and the rename refuses a name in use; that branch goes first.
    if any(b["name"] == name and not b.get("isMain") for b in branches):
        client.post("api/project_branches/delete", project=project, branch=name)
    client.post("api/project_branches/rename", project=project, name=name)
    return str(current)


def check(client: Client, config: Config) -> Report:
    """Compare gate and profiles on the server with the config without changing anything."""
    report = Report()
    project = _cloud_project(client) if client.is_cloud else ""
    found = _gate(client, config.gate_name)
    if found is None:
        report.problems.append(f"gate '{config.gate_name}' is missing")
    else:
        have = {c["metric"]: c for c in found[1]}
        for want in config.conditions:
            cond = have.pop(want.metric, None)
            if cond is None:
                report.problems.append(
                    f"gate '{config.gate_name}': condition {want.metric} missing"
                )
            elif (cond.get("op"), cond["error"]) != (want.op, want.error):
                report.problems.append(
                    f"gate '{config.gate_name}': condition {want.metric} {cond.get('op')} "
                    f"{cond['error']} differs (want {want.op} {want.error})"
                )
        report.problems += [f"gate '{config.gate_name}': extra condition {m}" for m in sorted(have)]
        if not _gate_is_default(client, config.gate_name):
            if client.is_cloud:
                report.gaps.append(GATE_NOT_DEFAULT)
            else:
                report.problems.append(f"gate '{config.gate_name}' is not the default gate")
        if client.is_cloud:
            _check_project_gate(client, config.gate_name, project, report)

    used: dict[str, dict[str, Any]] = {}
    if client.is_cloud:
        found_used = client.get_json("api/qualityprofiles/search", project=project)["profiles"]
        used = {p["language"]: p for p in found_used}

    want_parent = config.parent_for(client)
    available = _languages(client)
    for language, xml_path in config.profiles.items():
        if language not in available:
            report.skipped_languages.append(language)
            continue
        profile = _profile(client, language, config.profile_name)
        if profile is None:
            report.problems.append(f"profile {language}: '{config.profile_name}' is missing")
            continue
        if client.is_cloud:
            _check_project_profile(used.get(language), language, config.profile_name, report)
        parent = profile.get("parentName")
        if parent != want_parent:
            report.problems.append(
                f"profile {language}: parent is {parent!r}, want {want_parent!r}"
            )
            continue
        delta = _active_rules(client, language, config.profile_name) - _active_rules(
            client, language, want_parent
        )
        want_rules = xml_rules(xml_path.read_text(encoding="utf-8"))
        if delta != want_rules:
            report.problems.append(
                f"profile {language}: activated beyond parent {sorted(delta)}, "
                f"want {sorted(want_rules)}"
            )
    return report


def _check_project_gate(client: Client, name: str, project: str, report: Report) -> None:
    """Report the gate the cloud project uses: a built-in one is a plan gap, another a problem."""
    by_project = client.get_json("api/qualitygates/get_by_project", project=project)
    used = by_project["qualityGate"]["name"]
    if used == name:
        return
    gates = client.get_json("api/qualitygates/list")["qualitygates"]
    if any(g["name"] == used and g.get("isBuiltIn") for g in gates):
        report.gaps.append(f"{GATE_NOT_ON_PROJECT}: project uses the built-in gate '{used}'")
    else:
        report.problems.append(f"gate '{name}': project uses gate '{used}'")


def _check_project_profile(
    used: dict[str, Any] | None, language: str, name: str, report: Report
) -> None:
    """Report the profile the cloud project uses for `language` like _check_project_gate."""
    if used is None:
        report.problems.append(f"profile {language}: project uses no profile, want '{name}'")
    elif used["name"] == name:
        return
    elif used.get("isBuiltIn"):
        report.gaps.append(
            f"profile {language}: {PROFILE_NOT_ON_PROJECT}: project uses the built-in profile "
            f"'{used['name']}'"
        )
    else:
        report.problems.append(
            f"profile {language}: project uses profile '{used['name']}', want '{name}'"
        )


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (`apply`, `check`, `main-branch`, `ce-error`); returns exit code."""
    parser = argparse.ArgumentParser(description="Apply or check the SonarQube quality setup.")
    parser.add_argument("command", choices=("apply", "check", "main-branch", "ce-error"))
    parser.add_argument("--url", default=os.environ.get("SONAR_HOST_URL", "http://127.0.0.1:9000"))
    parser.add_argument("--organization", default=os.environ.get("SONAR_ORGANIZATION") or None)
    parser.add_argument("--project", default=os.environ.get("SONAR_PROJECT_KEY") or None)
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--report-task", type=Path, default=DEFAULT_REPORT_TASK)
    args = parser.parse_args(argv)
    token = os.environ.get("SONAR_TOKEN", "")
    if not token:
        print(
            "quality_sync: SONAR_TOKEN is not set (see docs/quality/sonarqube-local.md)",
            file=sys.stderr,
        )
        return 2
    client = Client(args.url, token, args.organization, args.project)
    if args.command == "main-branch":
        return _main_branch(client)
    if args.command == "ce-error":
        return _ce_error(client, args.report_task)
    try:
        config = load_config(args.config)
        report = apply(client, config) if args.command == "apply" else check(client, config)
    except (SonarError, LookupError, urllib.error.URLError) as exc:
        print(f"quality_sync: {exc}", file=sys.stderr)
        return 1
    if report.skipped_languages:
        print(f"quality_sync: not available on this server: {', '.join(report.skipped_languages)}")
    for gap in report.gaps:
        print(f"quality_sync: gap: {gap}")
    for problem in report.problems:
        print(f"quality_sync: {problem}")
    if report.problems:
        return 1
    print(f"quality_sync: {args.command} ok for {args.url}")
    return 0


def ce_task_id(report_task: Path) -> str | None:
    """Return the ceTaskId the scanner wrote to `report_task`, or None without one."""
    try:
        lines = report_task.read_text(encoding="utf-8").splitlines()
    except OSError:
        return None
    return next((ln.split("=", 1)[1].strip() for ln in lines if ln.startswith("ceTaskId=")), None)


def _ce_error(client: Client, report_task: Path) -> int:
    """Run `ce-error`: print status and error message of the scan's background task."""
    task_id = ce_task_id(report_task)
    if not task_id:
        print(f"quality_sync: no ceTaskId in {report_task}", file=sys.stderr)
        return 1
    try:
        task = client.get_json("api/ce/task", id=task_id)["task"]
    except (SonarError, LookupError, urllib.error.URLError) as exc:
        print(f"quality_sync: {exc}", file=sys.stderr)
        return 1
    status = task.get("status")
    if status not in ("FAILED", "CANCELED"):
        # The task went through; the scanner failed for another reason (such as the gate),
        # which its own output above names.
        print(f"quality_sync: background task {task_id} {status}, not the cause")
        return 0
    reason = str(task.get("errorMessage", ""))[:_MAX_ERROR_CHARS]
    print(f"quality_sync: background task {task_id} {status}: {reason or 'no message'}")
    return 0


def _main_branch(client: Client) -> int:
    """Run `main-branch`: name the project's main branch `main`."""
    try:
        old = ensure_main_branch(client, "main")
    except (SonarError, LookupError, urllib.error.URLError) as exc:
        print(f"quality_sync: {exc}", file=sys.stderr)
        return 1
    if old is None:
        print("quality_sync: main branch is already 'main'")
    else:
        print(f"quality_sync: main branch renamed from '{old}' to 'main'")
    return 0


if __name__ == "__main__":
    sys.exit(main())
