# SPDX-License-Identifier: Apache-2.0
"""Apply and check the project's SonarQube quality gate and quality profiles.

The repository holds the truth (quality/sonar/): quality-gate.json, profiles.json and
one quality-profile-<language>.xml per language with the rules activated IN ADDITION to
the parent profile. The same files are applied to the local SonarQube Community Build
and to SonarQube Cloud; `check` against both proves they enforce the same gate and the
same profile delta (docs/quality/sonarqube-local.md).

Usage (token only from the environment variable SONAR_TOKEN, never printed):
  quality_sync.py apply --url http://127.0.0.1:9000
  quality_sync.py check --url https://sonarcloud.io --organization <org>

Languages the server does not know (Dart and Swift on the Community Build) are skipped
and listed; the Cloud check covers them.

SonarQube Cloud refuses a custom default gate (HTTP 403 on set_as_default) on plans that
do not include it. In cloud mode that is a reported gap, not a failure: apply still
applies the profiles, and check reports the gap while every other difference fails.
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
_RULE = re.compile(r"<repositoryKey>([^<]+)</repositoryKey>\s*<key>([^<]+)</key>")
_TIMEOUT_S = 60
_MAX_ERROR_CHARS = 300
GATE_NOT_DEFAULT = "gate not default: plan does not allow it"


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

    def __init__(self, url: str, token: str, organization: str | None = None) -> None:
        """Create a client; raises SonarError for a URL scheme other than http or https."""
        if urllib.parse.urlsplit(url).scheme not in ("http", "https"):
            raise SonarError(f"unsupported URL scheme in {url!r}; use http or https")
        self._url = url.rstrip("/")
        self._token = token
        self._organization = organization
        self.is_cloud = organization is not None

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
            reason = _error_messages(exc.read())
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


def _profile(client: Client, language: str, name: str) -> dict[str, Any] | None:
    found = client.get_json("api/qualityprofiles/search", language=language, qualityProfile=name)
    return next((p for p in found["profiles"] if p["name"] == name), None)


def _active_rules(client: Client, language: str, name: str) -> set[str]:
    xml = client.get("api/qualityprofiles/backup", language=language, qualityProfile=name)
    return xml_rules(xml.decode("utf-8"))


def apply(client: Client, config: Config) -> Report:
    """Create or update gate and profiles so that the server matches the config."""
    report = Report()
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
        client.post(
            "api/qualityprofiles/set_default", language=language, qualityProfile=config.profile_name
        )
    return report


def check(client: Client, config: Config) -> Report:
    """Compare gate and profiles on the server with the config without changing anything."""
    report = Report()
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


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (`apply` or `check`); returns the process exit code."""
    parser = argparse.ArgumentParser(description="Apply or check the SonarQube quality setup.")
    parser.add_argument("command", choices=("apply", "check"))
    parser.add_argument("--url", default=os.environ.get("SONAR_HOST_URL", "http://127.0.0.1:9000"))
    parser.add_argument("--organization", default=os.environ.get("SONAR_ORGANIZATION") or None)
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    args = parser.parse_args(argv)
    token = os.environ.get("SONAR_TOKEN", "")
    if not token:
        print(
            "quality_sync: SONAR_TOKEN is not set (see docs/quality/sonarqube-local.md)",
            file=sys.stderr,
        )
        return 2
    client = Client(args.url, token, args.organization)
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


if __name__ == "__main__":
    sys.exit(main())
