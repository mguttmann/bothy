# SPDX-License-Identifier: Apache-2.0
"""In-memory stand-in for the subset of the SonarQube Web API that quality_sync uses.

Endpoint names and parameters follow the public Web API description
(api/webservices/list of a SonarQube Server, retrieved 2026-10-10). With `cloud=True`
it follows SonarQube Cloud instead (https://sonarcloud.io/api/webservices/list,
retrieved 2026-10-13): create_condition needs `gateId` and set_as_default needs `id`.
`gate_default_forbidden=True` answers set_as_default with HTTP 403, as SonarQube Cloud does
for a custom default gate on a plan that does not include it. `profile_default_forbidden` and
`profile_project_forbidden` answer set_default and add_project with HTTP 400 "Organization ...
is not allowed to perform this action", as SonarQube Cloud answered set_default on a
sonar-config run of 2026-10-14; `gate_select_forbidden` answers select with HTTP 403
"Organization ... is not allowed to modify Quality gates", as it did on a later run that day.
`project_permission_denied` answers add_project with HTTP 403 "Insufficient privileges", as
for a token without the needed permission. A project uses the profile or gate assigned to it,
otherwise the default one; the built-in gate is "Sonar way". `main_branch` is the name of the
project's main branch (api/project_branches/list and rename); `branch_rename_refused` answers
rename with HTTP 400 "Organization ... is not allowed to perform this action".
"""

from __future__ import annotations

import json
import re
import threading
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import parse_qs, urlparse


@dataclass
class Profile:
    key: str
    name: str
    language: str
    builtin: bool = False
    parent: str | None = None
    own_rules: set[str] = field(default_factory=set)
    default: bool = False


@dataclass
class State:
    languages: tuple[str, ...] = ("go", "py", "ts", "js", "docker", "kotlin")
    cloud: bool = False
    builtin_profile: str = "Sonar way"
    gates: dict[str, list[dict[str, str]]] = field(default_factory=dict)
    gate_ids: dict[str, str] = field(default_factory=dict)
    default_gate: str | None = None
    gate_default_forbidden: bool = False
    profile_default_forbidden: bool = False
    profile_project_forbidden: bool = False
    gate_select_forbidden: bool = False
    project_permission_denied: bool = False
    project_gate: str | None = None
    main_branch: str = "master"
    branch_rename_refused: bool = False
    project_profiles: dict[str, str] = field(default_factory=dict)
    profiles: list[Profile] = field(default_factory=list)
    calls: list[tuple[str, str, dict[str, str]]] = field(default_factory=list)
    tokens: list[str] = field(default_factory=list)
    next_id: int = 1

    def __post_init__(self) -> None:
        for lang in self.languages:
            self.profiles.append(
                Profile(
                    f"sw-{lang}",
                    self.builtin_profile,
                    lang,
                    builtin=True,
                    own_rules={f"{lang}:S100", f"{lang}:S107"},
                    default=True,
                )
            )

    def profile(self, language: str, name: str) -> Profile | None:
        return next((p for p in self.profiles if p.language == language and p.name == name), None)

    def active_rules(self, p: Profile) -> set[str]:
        rules = set(p.own_rules)
        if p.parent:
            parent = self.profile(p.language, p.parent)
            if parent:
                rules |= self.active_rules(parent)
        return rules


def _backup_xml(state: State, p: Profile) -> str:
    rules = "".join(
        f"<rule><repositoryKey>{r.split(':')[0]}</repositoryKey><key>{r.split(':')[1]}</key></rule>"
        for r in sorted(state.active_rules(p))
    )
    return (
        f"<?xml version='1.0' encoding='UTF-8'?><profile><name>{p.name}</name>"
        f"<language>{p.language}</language><rules>{rules}</rules></profile>"
    )


def _make_handler(state: State) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args: Any) -> None:  # silence test output
            return

        def _reply(self, status: int, body: Any, ctype: str = "application/json") -> None:
            data = body.encode() if isinstance(body, str) else json.dumps(body).encode()
            self.send_response(status)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def _params(self) -> dict[str, str]:
            parsed = urlparse(self.path)
            params = {k: v[0] for k, v in parse_qs(parsed.query).items()}
            if self.command == "POST":
                length = int(self.headers.get("Content-Length", "0"))
                raw = self.rfile.read(length)
                ctype = self.headers.get("Content-Type", "")
                if ctype.startswith("multipart/form-data"):
                    m = re.search(rb"<profile>.*</profile>", raw, re.DOTALL)
                    params["backup"] = m.group(0).decode() if m else ""
                else:
                    params.update({k: v[0] for k, v in parse_qs(raw.decode()).items()})
            return params

        def do_GET(self) -> None:
            self._dispatch()

        def do_POST(self) -> None:
            self._dispatch()

        def _dispatch(self) -> None:
            path = urlparse(self.path).path
            params = self._params()
            state.calls.append((self.command, path, params))
            state.tokens.append(self.headers.get("Authorization", ""))
            handler = ROUTES.get((self.command, path))
            if handler is None:
                self._reply(404, {"errors": [{"msg": f"unknown {path}"}]})
                return
            status, body, ctype = handler(state, params)
            self._reply(status, body, ctype)

    return Handler


def _languages(state: State, _: dict[str, str]) -> tuple[int, Any, str]:
    return 200, {"languages": [{"key": k, "name": k} for k in state.languages]}, "application/json"


_BAD_REQUEST = (400, {"errors": [{"msg": "missing parameter"}]}, "application/json")
_NOT_ALLOWED = (
    400,
    {"errors": [{"msg": "Organization 'example-org' is not allowed to perform this action"}]},
    "application/json",
)
BUILTIN_GATE = "Sonar way"


def _gate_name(state: State, p: dict[str, str], name_key: str, id_key: str) -> str | None:
    """Resolve the gate a call addresses: by id on the cloud, by name on the server."""
    if state.cloud:
        return next((n for n, i in state.gate_ids.items() if i == p.get(id_key)), None)
    return p.get(name_key)


def _gate_show(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    name = p.get("name")
    if name not in state.gates:
        return 404, {"errors": [{"msg": "No quality gate"}]}, "application/json"
    body = {"id": state.gate_ids[name], "name": name, "conditions": state.gates[name]}
    return 200, body, "application/json"


def _gate_create(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    state.gates[p["name"]] = []
    state.gate_ids[p["name"]] = str(100 + len(state.gate_ids))
    return 200, {"id": state.gate_ids[p["name"]], "name": p["name"]}, "application/json"


def _cond_create(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    gate = _gate_name(state, p, "gateName", "gateId")
    if gate not in state.gates:
        return _BAD_REQUEST
    cond = {
        "id": str(state.next_id),
        "metric": p["metric"],
        "op": p.get("op", "LT"),
        "error": p["error"],
    }
    state.next_id += 1
    state.gates[gate].append(cond)
    return 200, cond, "application/json"


def _cond_update(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    for conds in state.gates.values():
        for c in conds:
            if c["id"] == p["id"]:
                c.update(metric=p["metric"], op=p.get("op", c["op"]), error=p["error"])
    return 204, "", "text/plain"


def _cond_delete(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    for name, conds in state.gates.items():
        state.gates[name] = [c for c in conds if c["id"] != p["id"]]
    return 204, "", "text/plain"


def _gate_list(state: State, _: dict[str, str]) -> tuple[int, Any, str]:
    gates = [
        {"id": state.gate_ids[n], "name": n, "isDefault": n == state.default_gate}
        for n in state.gates
    ]
    builtin = {"id": "1", "name": BUILTIN_GATE, "isBuiltIn": True}
    gates.append({**builtin, "isDefault": state.default_gate is None})
    return 200, {"qualitygates": gates}, "application/json"


def _gate_default(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if state.gate_default_forbidden:
        return 403, {"errors": [{"msg": "Insufficient privileges"}]}, "application/json"
    gate = _gate_name(state, p, "name", "id")
    if gate not in state.gates:
        return _BAD_REQUEST
    state.default_gate = gate
    return 204, "", "text/plain"


def _gate_select(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if state.gate_select_forbidden:
        msg = "Organization 'example-org' is not allowed to modify Quality gates"
        return 403, {"errors": [{"msg": msg}]}, "application/json"
    gate = _gate_name(state, p, "gateName", "gateId")
    if gate not in state.gates or not p.get("projectKey"):
        return _BAD_REQUEST
    state.project_gate = gate
    return 204, "", "text/plain"


def _gate_by_project(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if not p.get("project"):
        return _BAD_REQUEST
    name = state.project_gate or state.default_gate or BUILTIN_GATE
    is_default = name == state.default_gate
    gate = {"id": state.gate_ids.get(name, "1"), "name": name, "default": is_default}
    return 200, {"qualityGate": gate}, "application/json"


def _project_profile(state: State, language: str) -> Profile | None:
    name = state.project_profiles.get(language)
    if name is not None:
        return state.profile(language, name)
    return next((x for x in state.profiles if x.language == language and x.default), None)


def _qp_search(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if p.get("project"):
        used = [_project_profile(state, lang) for lang in state.languages]
        found = [x for x in used if x is not None]
    else:
        found = [
            x
            for x in state.profiles
            if x.language == p.get("language", x.language)
            and x.name == p.get("qualityProfile", x.name)
        ]
    return (
        200,
        {
            "profiles": [
                {
                    "key": x.key,
                    "name": x.name,
                    "language": x.language,
                    "isBuiltIn": x.builtin,
                    "isDefault": x.default,
                    "parentName": x.parent,
                }
                for x in found
            ]
        },
        "application/json",
    )


def _qp_create(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    prof = Profile(f"k-{p['language']}-{len(state.profiles)}", p["name"], p["language"])
    state.profiles.append(prof)
    return 200, {"profile": {"key": prof.key}}, "application/json"


def _qp_parent(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    prof = state.profile(p["language"], p["qualityProfile"])
    if prof is None or state.profile(p["language"], p["parentQualityProfile"]) is None:
        return 404, {"errors": [{"msg": "not found"}]}, "application/json"
    prof.parent = p["parentQualityProfile"]
    return 204, "", "text/plain"


def _qp_restore(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    xml = p["backup"]
    name = re.search(r"<name>(.*?)</name>", xml).group(1)  # type: ignore[union-attr]
    lang = re.search(r"<language>(.*?)</language>", xml).group(1)  # type: ignore[union-attr]
    prof = state.profile(lang, name)
    if prof is None:
        prof = Profile(f"r-{lang}", name, lang)
        state.profiles.append(prof)
    for repo, key in re.findall(r"<repositoryKey>(.*?)</repositoryKey>\s*<key>(.*?)</key>", xml):
        prof.own_rules.add(f"{repo}:{key}")
    return 200, {"profile": {"key": prof.key}}, "application/json"


def _qp_add_project(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if state.profile_project_forbidden:
        return _NOT_ALLOWED
    if state.project_permission_denied:
        return 403, {"errors": [{"msg": "Insufficient privileges"}]}, "application/json"
    if not p.get("project") or state.profile(p["language"], p["qualityProfile"]) is None:
        return _BAD_REQUEST
    state.project_profiles[p["language"]] = p["qualityProfile"]
    return 204, "", "text/plain"


def _qp_default(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if state.profile_default_forbidden:
        return _NOT_ALLOWED
    for prof in state.profiles:
        if prof.language == p["language"]:
            prof.default = prof.name == p["qualityProfile"]
    return 204, "", "text/plain"


def _qp_backup(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    prof = state.profile(p["language"], p["qualityProfile"])
    if prof is None:
        return 404, {"errors": [{"msg": "not found"}]}, "application/json"
    return 200, _backup_xml(state, prof), "application/xml"


def _branches(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if not p.get("project"):
        return _BAD_REQUEST
    others = [{"name": "feature", "isMain": False, "type": "SHORT"}]
    main = {"name": state.main_branch, "isMain": True, "type": "LONG"}
    return 200, {"branches": [*others, main]}, "application/json"


def _branch_rename(state: State, p: dict[str, str]) -> tuple[int, Any, str]:
    if state.branch_rename_refused:
        return _NOT_ALLOWED
    if not p.get("project") or not p.get("name"):
        return _BAD_REQUEST
    state.main_branch = p["name"]
    return 204, "", "text/plain"


ROUTES = {
    ("GET", "/api/languages/list"): _languages,
    ("GET", "/api/qualitygates/show"): _gate_show,
    ("GET", "/api/qualitygates/list"): _gate_list,
    ("POST", "/api/qualitygates/create"): _gate_create,
    ("POST", "/api/qualitygates/create_condition"): _cond_create,
    ("POST", "/api/qualitygates/update_condition"): _cond_update,
    ("POST", "/api/qualitygates/delete_condition"): _cond_delete,
    ("POST", "/api/qualitygates/set_as_default"): _gate_default,
    ("POST", "/api/qualitygates/select"): _gate_select,
    ("GET", "/api/qualitygates/get_by_project"): _gate_by_project,
    ("GET", "/api/qualityprofiles/search"): _qp_search,
    ("POST", "/api/qualityprofiles/create"): _qp_create,
    ("POST", "/api/qualityprofiles/change_parent"): _qp_parent,
    ("POST", "/api/qualityprofiles/restore"): _qp_restore,
    ("POST", "/api/qualityprofiles/set_default"): _qp_default,
    ("POST", "/api/qualityprofiles/add_project"): _qp_add_project,
    ("GET", "/api/qualityprofiles/backup"): _qp_backup,
    ("GET", "/api/project_branches/list"): _branches,
    ("POST", "/api/project_branches/rename"): _branch_rename,
}


@contextmanager
def running(state: State) -> Iterator[str]:
    server = ThreadingHTTPServer(("127.0.0.1", 0), _make_handler(state))
    thread = threading.Thread(
        target=server.serve_forever, kwargs={"poll_interval": 0.05}, daemon=True
    )
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}"
    finally:
        server.shutdown()
        server.server_close()
