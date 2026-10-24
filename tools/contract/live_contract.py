# SPDX-License-Identifier: Apache-2.0
"""G7: Schemathesis against the server of the working tree (06 "Contract tests and gate").

Run only by ``TestG7_SchemathesisAllEnabledOperations`` (``make contract-test``), which starts the
harness server with the matrix world and the credential broker and sets:

``BOTHY_CONTRACT_BASE_URL``
    ``APIBase`` of the server (``http://127.0.0.1:<port>/api/v1``).
``BOTHY_CONTRACT_BROKER_URL``
    origin of the credential broker.
``BOTHY_CONTRACT_OPERATIONS``, ``BOTHY_CONTRACT_SPEC``
    paths of ``services/server/api/operations.yaml`` and ``docs/api/openapi.yaml``.
``CONTRACT_MAX_EXAMPLES``, ``CONTRACT_SEED``
    examples per operation and the seed of the run.
``BOTHY_CONTRACT_STEP_UP_MAX_AGE_S``, ``BOTHY_CONTRACT_ACCESS_TOKEN_TTL_S``,
``BOTHY_CONTRACT_NONCE_TTL_S``
    ``auth.step_up_max_age``, ``auth.access_token_ttl`` and ``auth.dpop_nonce_ttl`` of the server.
``BOTHY_CONTRACT_UPLOAD_CHANNEL``
    a channel where the plain user may upload (positive cases of the upload operations).
``BOTHY_CONTRACT_PUSH_APP_ID``, ``BOTHY_CONTRACT_PREVIEW_MESSAGE``
    the APNs topic the server enables (``registerPush``) and a message both passes may read
    (positive cases of ``getPushPreview``).

The file name does not match ``test_*.py``, so the default ``pytest`` run never collects it.

Tests: one Schemathesis test per enabled operation and pass (owner, plain user Alex), the
stateful phase over the links Schemathesis infers, SEC-T06 (unknown member, duplicate member) and
a coverage check at the end of the session that fails when an enabled operation was not called in
a pass. Every failure prints a ``curl`` reproduction without credentials and the seed.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import re
import shutil
import tempfile
from collections.abc import Iterator, Mapping
from pathlib import Path
from typing import Any
from urllib.parse import urlencode

import hypothesis
import hypothesis.errors
import pytest
import schemathesis
from auth import PREVIEW_SCHEME, PassAuth, problem_code, provider_class, uuid7, without_host
from broker_client import BrokerClient
from checks import (
    Answer,
    ContractError,
    correlation_id,
    no_json_null,
    not_found_equality,
    problem_codes,
    problem_format,
)
from operations import HTTP_METHODS, enabled_operations, load_spec
from reporting import curl_command, failure_report
from schemathesis import GenerationMode
from schemathesis.checks import not_a_server_error
from schemathesis.core.failures import FailureGroup
from schemathesis.core.parameters import ParameterLocation
from schemathesis.generation.stateful import run_state_machine_as_test
from schemathesis.specs.openapi.checks import (
    content_type_conformance,
    ignored_auth,
    negative_data_rejection,
    response_headers_conformance,
    response_schema_conformance,
    status_code_conformance,
)

BASE_URL = os.environ["BOTHY_CONTRACT_BASE_URL"].rstrip("/")
BROKER = BrokerClient(os.environ["BOTHY_CONTRACT_BROKER_URL"])
SPEC_PATH = Path(os.environ["BOTHY_CONTRACT_SPEC"])
SPEC = load_spec(SPEC_PATH)
ENABLED = enabled_operations(Path(os.environ["BOTHY_CONTRACT_OPERATIONS"]), SPEC)
MAX_EXAMPLES = int(os.environ.get("CONTRACT_MAX_EXAMPLES", "100"))
SEED = int(os.environ["CONTRACT_SEED"])
CODES = problem_codes(SPEC)
KEY_DIR = Path(tempfile.mkdtemp(prefix="bothy-contract-keys-"))
# submitCspReport takes the legacy CSP report type besides application/reports+json (M6-03,
# 06 "Media types"); both are JSON, so Schemathesis serializes them like application/json.
schemathesis.serializer.alias("application/csp-report", "application/json")
# A channel the plain user may upload to (Support/Allgemein of the matrix world, M3-03); the
# upload operations of a positive case run against a fresh session there, and deleteFile (M3-08)
# deletes such a fresh own file.
UPLOAD_CHANNEL = os.environ.get("BOTHY_CONTRACT_UPLOAD_CHANNEL", "")
UPLOAD_OPS = frozenset({"uploadPart", "getUpload", "completeUpload", "abortUpload", "deleteFile"})
# The APNs topic of the server under test (push.apns.topics) for registerPush, and a message of
# the matrix world that the owner and Alex may read, so getPushPreview can answer 200 (M3-22).
PUSH_APP_ID = os.environ.get("BOTHY_CONTRACT_PUSH_APP_ID", "")
PREVIEW_MESSAGE = os.environ.get("BOTHY_CONTRACT_PREVIEW_MESSAGE", "")

_UUID_PARAM = re.compile(r"\{([a-z_]+)\}")


def _resolve(node: Any) -> Any:
    """Follow a local ``$ref`` of the OpenAPI document."""
    while isinstance(node, dict) and "$ref" in node:
        target: Any = SPEC
        for part in str(node["$ref"]).lstrip("#/").split("/"):
            target = target[part]
        node = target
    return node


def _operations() -> dict[str, dict[str, Any]]:
    """Return operationId -> facts of every enabled operation."""
    out: dict[str, dict[str, Any]] = {}
    for path, item in SPEC["paths"].items():
        shared = [_resolve(p) for p in item.get("parameters", [])]
        for method, op in item.items():
            if method not in HTTP_METHODS or op.get("operationId") not in ENABLED:
                continue
            params = shared + [_resolve(p) for p in op.get("parameters", [])]
            body = _resolve(op.get("requestBody")) or {}
            json_body = (body.get("content") or {}).get("application/json")
            example = None
            if json_body:
                examples = json_body.get("examples") or {}
                first: dict[str, Any] = next(iter(examples.values()), {})
                example = first.get("value") if examples else json_body.get("example")
            out[op["operationId"]] = {
                "method": method.upper(),
                "path": path,
                "anonymous": op.get("security") == [],
                "dpop_param": any(
                    p.get("in") == "header" and p.get("name") == "DPoP" for p in params
                ),
                "params": params,
                "json_example": example,
                # The token check of an incoming webhook runs before the body is read (11 8.4):
                # without a valid webhook token every call answers the uniform 404.
                "token_gated": op.get("x-authz-action") == "webhook_in.post",
                # getPushPreview takes the preview token of a registration, no access token.
                "preview_token": any(
                    PREVIEW_SCHEME in requirement for requirement in op.get("security") or []
                ),
            }
    return out


OPERATIONS = _operations()
BY_LABEL = {f"{o['method']} {o['path']}": op_id for op_id, o in OPERATIONS.items()}


def _state(kind: str) -> PassAuth:
    return PassAuth(
        broker=BROKER,
        kind=kind,
        api_base=BASE_URL,
        key_dir=KEY_DIR,
        step_up_max_age=float(os.environ.get("BOTHY_CONTRACT_STEP_UP_MAX_AGE_S", "300")),
        access_token_ttl=float(os.environ.get("BOTHY_CONTRACT_ACCESS_TOKEN_TTL_S", "600")),
        nonce_ttl=float(os.environ.get("BOTHY_CONTRACT_NONCE_TTL_S", "300")),
        push_app_id=PUSH_APP_ID,
    )


PASSES = {"owner": _state("owner"), "user": _state("user")}


def _operation_id(case: Any) -> str:
    return BY_LABEL[f"{case.operation.method.upper()} {case.operation.path}"]


def _negative(case: Any, location: ParameterLocation) -> bool:
    meta = case.meta
    component = meta.components.get(location) if meta is not None else None
    return bool(component is not None and component.mode.is_negative)


def _auth_headers(state: PassAuth) -> Any:
    """Return the function that computes the auth headers of a case for ``state``."""

    def headers(case: Any) -> dict[str, str]:
        facts = OPERATIONS[_operation_id(case)]
        url = BASE_URL + case.formatted_path
        if facts["preview_token"]:
            out = state.preview_headers(case.method, url)
        else:
            out = state.headers(
                case.method, url, authenticated=not facts["anonymous"], proof=facts["dpop_param"]
            )
        current = {k.lower(): v for k, v in (case.headers or {}).items()}
        if "idempotency-key" in current and not _negative(case, ParameterLocation.HEADER):
            # A generated UUID is rarely a v7; the server would refuse every positive case.
            out["Idempotency-Key"] = uuid7()
        return out

    return headers


def _load(kind: str) -> Any:
    """Load openapi.yaml restricted to the enabled operations, with the auth of the pass."""
    schema: Any = schemathesis.openapi.from_path(SPEC_PATH, config=schemathesis.Config(seed=SEED))
    schema.config.update(base_url=BASE_URL, request_timeout=60)
    schema.config.generation.update(
        modes=[GenerationMode.POSITIVE, GenerationMode.NEGATIVE],
        max_examples=MAX_EXAMPLES,
        database="none",
        deterministic=False,
        with_security_parameters=False,
    )
    schema = schema.include(operation_id=ENABLED)
    schema.auth(refresh_interval=None)(provider_class(PASSES[kind], _auth_headers(PASSES[kind])))
    return schema


SCHEMAS = {"owner": _load("owner"), "user": _load("user")}

# The built-in checks of the pinned version that G7 runs (brief M1-71 design note G7 6).
BUILTIN_CHECKS = [
    not_a_server_error,
    status_code_conformance,
    content_type_conformance,
    response_headers_conformance,
    response_schema_conformance,
    negative_data_rejection,
    ignored_auth,
]


def _answer(response: Any) -> Answer:
    headers = {
        k: ", ".join(v) if isinstance(v, list) else str(v) for k, v in response.headers.items()
    }
    return Answer(response.status_code, headers, bytes(response.content))


def _request_parts(response: Any) -> tuple[str, str, dict[str, str], bytes | None]:
    request = response.request
    body = request.body
    if isinstance(body, str):
        body = body.encode()
    return str(request.method), str(request.url), dict(request.headers), body


def _report(reason: str, response: Any, state: PassAuth) -> str:
    method, url, headers, body = _request_parts(response)
    key = state.session.key_file if state.session else KEY_DIR / "missing.jwk.json"
    return failure_report(reason, curl_command(method, url, headers, body, key_file=key), SEED)


def _alternate_url(case: Any) -> str | None:
    """Return the URL of ``case`` with fresh UUIDv7 values for its UUID path parameters, or None."""
    facts = OPERATIONS[_operation_id(case)]
    uuid_params = {
        p["name"]
        for p in facts["params"]
        if p.get("in") == "path" and _resolve(p.get("schema", {})).get("format") == "uuid"
    }
    if not uuid_params:
        return None
    values = dict(case.path_parameters or {})
    for name in uuid_params:
        values[name] = uuid7()
    path = _UUID_PARAM.sub(lambda m: str(values.get(m.group(1), m.group(0))), case.operation.path)
    query = urlencode(case.query or {}, doseq=True)
    return BASE_URL + path + ("?" + query if query else "")


def _check_not_found(case: Any, response: Any, state: PassAuth) -> None:
    # R-AZ-01 compares the 404 of one ID with that of fresh IDs; a malformed path parameter is no
    # ID, and its own path in instance would differ from theirs.
    if response.status_code != 404 or _negative(case, ParameterLocation.PATH):
        return
    url = _alternate_url(case)
    if url is None:
        return
    method, _, headers, body = _request_parts(response)
    headers = {
        k: v
        for k, v in headers.items()
        if k.lower() not in {"authorization", "dpop", "content-length"}
    }
    if "Idempotency-Key" in headers:
        headers["Idempotency-Key"] = uuid7()
    other = state.send(
        method, url, headers, body, authenticated=not OPERATIONS[_operation_id(case)]["anonymous"]
    )
    not_found_equality(_answer(response), Answer(other.status, other.headers, other.body))


def _project_checks(case: Any, response: Any, state: PassAuth) -> None:
    answer = _answer(response)
    problem_format(answer, CODES)
    correlation_id(answer)
    no_json_null(answer)
    _check_not_found(case, response, state)


def _prepare_upload(case: Any, state: PassAuth) -> None:
    """Point a positive case of an upload operation at a fresh session of the pass (M3-03).

    A random ``file_id`` only ever reaches the 404 of an unknown upload. For a case whose path is
    positive the pass announces an upload of the generated part (or a fixed byte string) and the
    case uses its file ID; uploadPart gets part number 1 and, unless the header is negative, the
    matching ``Content-Digest``. A pass that may not upload there keeps the generated case.
    """
    op = _operation_id(case)
    if op not in UPLOAD_OPS or not UPLOAD_CHANNEL or _negative(case, ParameterLocation.PATH):
        return
    body = b"contract"
    if op == "uploadPart" and isinstance(case.body, (bytes, bytearray)) and len(case.body) > 0:
        body = bytes(case.body)
    digest = hashlib.sha256(body).digest()
    resp = state.send(
        "POST",
        f"{BASE_URL}/channels/{UPLOAD_CHANNEL}/uploads",
        {"Content-Type": "application/json", "Idempotency-Key": uuid7()},
        json.dumps(
            {"name": "contract.bin", "size_bytes": len(body), "sha256": digest.hex()}
        ).encode(),
        authenticated=True,
    )
    if resp.status != 201:
        return
    params = dict(case.path_parameters or {})
    params["file_id"] = json.loads(resp.body)["file_id"]
    if op == "uploadPart":
        params["part_number"] = 1
        case.body = body
        if not _negative(case, ParameterLocation.HEADER):
            value = "sha-256=:" + base64.b64encode(digest).decode() + ":"
            case.headers = {**(case.headers or {}), "Content-Digest": value}
    case.path_parameters = params


def _prepare_push(case: Any, state: PassAuth) -> None:
    """Point positive cases of the push operations at real data of the pass (M3-22).

    unregisterPush with a positive path removes a fresh registration of the pass (``204``) instead
    of a random ID. getPushPreview with a positive query asks, for every second ``nid`` (its last
    hex digit even), for the readable message of the world, so the run sees ``200`` besides the
    ``404`` of an unknown message.
    """
    op = _operation_id(case)
    if op == "unregisterPush" and PUSH_APP_ID and not _negative(case, ParameterLocation.PATH):
        params = dict(case.path_parameters or {})
        params["registration_id"] = state.register_push().registration_id
        case.path_parameters = params
    elif (
        OPERATIONS[op]["preview_token"]
        and PREVIEW_MESSAGE
        and not _negative(case, ParameterLocation.QUERY)
    ):
        query = dict(case.query or {})
        nid = str(query.get("nid", ""))
        if nid and nid[-1] in "02468ace":
            query["message_id"] = PREVIEW_MESSAGE
            case.query = query


def _call(case: Any, state: PassAuth) -> Any:
    """Call ``case`` with fresh auth headers; repeat after a nonce, renewal or step-up answer."""
    _prepare_upload(case, state)
    _prepare_push(case, state)
    response = None
    case.headers = without_host(case.headers or {})
    for _ in range(3):
        if not schemathesis_negated_auth(case):
            case.headers = {**(case.headers or {}), **_auth_headers(state)(case)}
        response = case.call()
        action = state.observe(response.status_code, _answer(response).headers, response.content)
        if action is None:
            break
        if action == "step_up":
            state.step_up()
    return response


def schemathesis_negated_auth(case: Any) -> bool:
    """Whether Schemathesis negated a security parameter of ``case`` (auth stays as generated)."""
    return bool(case.operation.schema.is_security_param_negated(case))


# Answers per pass and status class, printed at the end of the session (report of M1-71).
STATS: dict[str, dict[str, int]] = {"owner": {}, "user": {}, "stateful": {}}
# Answers of a preview-token operation per pass that only an accepted preview token reaches (200,
# or 404 for the message after the token, session and proof checks passed).
PREVIEW_ACCEPTED: dict[str, int] = {"owner": 0, "user": 0}


def _count(kind: str, status: int) -> None:
    bucket = STATS[kind]
    key = f"{status // 100}xx"
    bucket[key] = bucket.get(key, 0) + 1
    bucket["cases"] = bucket.get("cases", 0) + 1


def run_case(case: Any, kind: str, called: dict[str, set[str]]) -> None:
    """Execute one generated case of pass ``kind`` with every check; record its operation."""
    state = PASSES[kind]
    called[kind].add(_operation_id(case))
    response = _call(case, state)
    _count(kind, response.status_code)
    if OPERATIONS[_operation_id(case)]["preview_token"] and response.status_code in (200, 404):
        PREVIEW_ACCEPTED[kind] += 1
    try:
        case.validate_response(response, checks=BUILTIN_CHECKS)
        _project_checks(case, response, state)
    except (AssertionError, FailureGroup) as exc:
        raise AssertionError(
            _report(f"{kind} pass, {_operation_id(case)}: {exc}", response, state)
        ) from None


@pytest.fixture(scope="session")
def called(pytestconfig: pytest.Config) -> Iterator[dict[str, set[str]]]:
    """Operations executed per pass; fails at the end when an enabled one was never called."""
    record: dict[str, set[str]] = {"owner": set(), "user": set()}
    yield record
    shutil.rmtree(KEY_DIR, ignore_errors=True)
    transitions = SCHEMAS["owner"].statistic.transitions.total
    summary = {
        "seed": SEED,
        "max_examples": MAX_EXAMPLES,
        "inferred_transitions": transitions,
        "preview_accepted": PREVIEW_ACCEPTED,
        **STATS,
    }
    # Through the terminal reporter, past the output capture, so the Go test log shows it.
    reporter = pytestconfig.pluginmanager.get_plugin("terminalreporter")
    if reporter is not None:
        reporter.write_line("G7 summary: " + json.dumps(summary, sort_keys=True))
    missing = {
        kind: sorted(set(ENABLED) - ops) for kind, ops in record.items() if set(ENABLED) - ops
    }
    if missing:
        pytest.fail(
            f"enabled operations never called: {json.dumps(missing)} (seed CONTRACT_SEED={SEED})"
        )
    # A preview-token operation that only ever answered 401 or 400 never proved the scheme.
    preview_ops = {op for op, f in OPERATIONS.items() if f["preview_token"]}
    unproven = sorted(
        kind for kind, ops in record.items() if ops & preview_ops and not PREVIEW_ACCEPTED[kind]
    )
    if unproven:
        pytest.fail(
            f"{PREVIEW_SCHEME} never accepted in the passes {unproven} (seed CONTRACT_SEED={SEED})"
        )


@SCHEMAS["user"].parametrize()  # type: ignore[untyped-decorator]
@hypothesis.seed(SEED)
def test_user_pass(case: Any, called: dict[str, set[str]]) -> None:
    """Every enabled operation as the plain user Alex of the matrix world."""
    run_case(case, "user", called)


@SCHEMAS["owner"].parametrize()  # type: ignore[untyped-decorator]
@hypothesis.seed(SEED)
def test_owner_pass(case: Any, called: dict[str, set[str]]) -> None:
    """Every enabled operation as the owner (main coverage)."""
    run_case(case, "owner", called)


class _OwnerMachine(SCHEMAS["owner"].as_state_machine()):  # type: ignore[misc]
    """Stateful phase as the owner over the links Schemathesis infers (openapi.yaml has none)."""

    def call(self, case: Any, **kwargs: Any) -> Any:
        """Call with fresh DPoP headers."""
        return _call(case, PASSES["owner"])

    def validate_response(
        self, response: Any, case: Any, additional_checks: Any = None, **kwargs: Any
    ) -> None:
        """Run the built-in and project checks."""
        state = PASSES["owner"]
        _count("stateful", response.status_code)
        try:
            case.validate_response(response, checks=BUILTIN_CHECKS)
            _project_checks(case, response, state)
        except (AssertionError, FailureGroup) as exc:
            report = _report(f"stateful, {_operation_id(case)}: {exc}", response, state)
            STATEFUL_FAILURES.append(report)
            raise AssertionError(report) from None


# Check failures of the stateful phase, kept so a replay that Hypothesis cannot repeat still
# reports them.
STATEFUL_FAILURES: list[str] = []


def test_stateful() -> None:
    """The stateful phase: sequences over the links Schemathesis infers, every check per step.

    The server keeps the state of earlier scenarios, so Hypothesis may find a replay that draws
    differently (``Flaky``); like the Schemathesis engine, that ends the phase without a failure
    unless a check failed before it.
    """
    settings = hypothesis.settings(
        max_examples=MAX_EXAMPLES,
        database=None,
        deadline=None,
        phases=[hypothesis.Phase.generate],
        suppress_health_check=list(hypothesis.HealthCheck),
    )
    try:
        run_state_machine_as_test(_OwnerMachine, settings=settings)
    except hypothesis.errors.Flaky:
        if STATEFUL_FAILURES:
            raise AssertionError(STATEFUL_FAILURES[0]) from None


def _json_operations() -> list[str]:
    return sorted(op for op, f in OPERATIONS.items() if isinstance(f["json_example"], dict))


def _target(op_id: str) -> tuple[str, str, dict[str, str]]:
    facts = OPERATIONS[op_id]

    def value(match: re.Match[str]) -> str:
        param: dict[str, Any] = next(
            (
                p
                for p in facts["params"]
                if p.get("in") == "path" and p.get("name") == match.group(1)
            ),
            {},
        )
        schema = _resolve(param.get("schema", {}))
        if schema.get("format") == "uuid":
            return uuid7()
        examples = schema.get("examples") or [schema.get("enum", ["x"])[0]]
        return str(examples[0])

    headers = {"Content-Type": "application/json", "If-Match": '"1"'}
    if any(p.get("name") == "Idempotency-Key" for p in facts["params"]):
        headers["Idempotency-Key"] = uuid7()
    return facts["method"], BASE_URL + _UUID_PARAM.sub(value, facts["path"]), headers


def _sec_call(op_id: str, body: bytes) -> tuple[Answer, str]:
    state = PASSES["owner"]
    method, url, headers = _target(op_id)
    resp = state.send(method, url, headers, body, authenticated=not OPERATIONS[op_id]["anonymous"])
    curl = curl_command(
        method,
        url,
        {**headers, "Authorization": "", "DPoP": ""},
        body,
        key_file=state.current().key_file,
    )
    return Answer(resp.status, resp.headers, resp.body), curl


def _token_gated_not_found(op_id: str, answer: Answer, curl: str) -> bool:
    """For an operation behind the webhook token check, require its uniform 404 and return True.

    The pass holds no webhook token, so the token check answers before the body is read (11 8.4);
    the body rules of these operations are tested in services/server/test/integrations.
    """
    if not OPERATIONS[op_id]["token_gated"]:
        return False
    if answer.status != 404 or problem_code(answer.body) != "not_found":
        raise ContractError(
            failure_report(
                f"{op_id}: without a webhook token answered {answer.status} {answer.body!r}",
                curl,
                SEED,
            )
        )
    return True


@pytest.mark.parametrize("op_id", _json_operations())
def test_sec_t06_unknown_field_rejected(op_id: str) -> None:
    """A valid body plus x_unknown_field gives 400 validation_failed (19 SEC-IN-1, PLAN W7)."""
    body = dict(OPERATIONS[op_id]["json_example"])
    body["x_unknown_field"] = True
    answer, curl = _sec_call(op_id, json.dumps(body).encode())
    if _token_gated_not_found(op_id, answer, curl):
        return
    errors = json.loads(answer.body).get("errors", []) if answer.status == 400 else []
    pointers = {e.get("pointer") for e in errors if isinstance(e, Mapping)}
    if (
        answer.status != 400
        or problem_code(answer.body) != "validation_failed"
        or "/x_unknown_field" not in pointers
    ):
        raise ContractError(
            failure_report(
                f"{op_id}: unknown member answered {answer.status} {answer.body!r}", curl, SEED
            )
        )


@pytest.mark.parametrize("op_id", _json_operations())
def test_sec_t06_duplicate_json_keys_rejected(op_id: str) -> None:
    """A body with one member twice gives 400 malformed_request (19 SEC-IN-2, 06 Errors)."""
    example: dict[str, Any] = OPERATIONS[op_id]["json_example"]
    name, value = next(iter(example.items())) if example else ("a", 1)
    members = [f"{json.dumps(name)}:{json.dumps(value)}"] * 2 + [
        f"{json.dumps(k)}:{json.dumps(v)}" for k, v in example.items() if k != name
    ]
    answer, curl = _sec_call(op_id, ("{" + ",".join(members) + "}").encode())
    if _token_gated_not_found(op_id, answer, curl):
        return
    if answer.status != 400 or problem_code(answer.body) != "malformed_request":
        raise ContractError(
            failure_report(
                f"{op_id}: duplicate member answered {answer.status} {answer.body!r}", curl, SEED
            )
        )
