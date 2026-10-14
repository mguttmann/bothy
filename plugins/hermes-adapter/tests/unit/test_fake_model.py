# SPDX-License-Identifier: Apache-2.0
"""The local fake model endpoint and the fake credentials."""

from __future__ import annotations

import http.client
import json
import re
import threading
import tomllib
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from hermes_harness.fake_model import CredentialKind, FakeCredentials, FakeModel, ScriptTurn

REPO = Path(__file__).resolve().parents[4]
TOOL_TURN = ScriptTurn(tool_calls=({"name": "todo_list", "arguments": {"todos": []}},))
TEXT_TURN = ScriptTurn(text=("Hel", "lo"))


@pytest.fixture
def creds() -> FakeCredentials:
    return FakeCredentials.generate()


@pytest.fixture
def model(creds: FakeCredentials) -> Iterator[FakeModel]:
    with FakeModel([TOOL_TURN, TEXT_TURN], creds, scenario_texts=["Please plan the trip."]) as m:
        yield m


def _post(
    model: FakeModel, path: str, body: dict[str, Any], headers: dict[str, str]
) -> tuple[int, str]:
    conn = http.client.HTTPConnection("127.0.0.1", model.port, timeout=30)
    try:
        conn.request(
            "POST", path, json.dumps(body), {"Content-Type": "application/json", **headers}
        )
        response = conn.getresponse()
        return response.status, response.read().decode("utf-8")
    finally:
        conn.close()


def _sse(text: str) -> list[tuple[str | None, str]]:
    events: list[tuple[str | None, str]] = []
    for block in text.strip().split("\n\n"):
        name = None
        data = ""
        for line in block.splitlines():
            if line.startswith("event: "):
                name = line[7:]
            elif line.startswith("data: "):
                data = line[6:]
        events.append((name, data))
    return events


def _openai_body(text: str = "Please plan the trip.") -> dict[str, Any]:
    return {
        "model": "fake-model",
        "stream": True,
        "messages": [
            {"role": "system", "content": "third party prompt"},
            {"role": "user", "content": text},
        ],
        "tools": [{"type": "function", "function": {"name": "todo_list"}}],
    }


def _anthropic_body() -> dict[str, Any]:
    return {
        "model": "fake-claude",
        "stream": True,
        "system": "third party prompt",
        "messages": [
            {"role": "user", "content": [{"type": "text", "text": "Please plan the trip."}]}
        ],
        "tools": [{"name": "todo_list"}],
    }


def test_openai_stream_replays_the_script(model: FakeModel, creds: FakeCredentials) -> None:
    auth = {"Authorization": f"Bearer {creds.api_key}"}
    status, text = _post(model, "/v1/chat/completions", _openai_body(), auth)
    assert status == 200
    chunks = [json.loads(d) for _, d in _sse(text) if d != "[DONE]"]
    assert _sse(text)[-1] == (None, "[DONE]")
    call = chunks[0]["choices"][0]["delta"]["tool_calls"][0]
    assert call["id"] == "call_1_1"
    assert call["function"]["name"] == "todo_list"
    assert json.loads(call["function"]["arguments"]) == {"todos": []}
    assert chunks[-1]["choices"][0]["finish_reason"] == "tool_calls"

    status, text = _post(model, "/v1/chat/completions", _openai_body(), auth)
    chunks = [json.loads(d) for _, d in _sse(text) if d != "[DONE]"]
    pieces = [c["choices"][0]["delta"].get("content") for c in chunks if c["choices"]]
    assert [p for p in pieces if p] == ["Hel", "lo"]
    assert chunks[-1]["choices"][0]["finish_reason"] == "stop"
    assert chunks[-1]["usage"]["total_tokens"] > 0
    assert [r["turn"] for r in model.requests()] == [1, 2]


def test_anthropic_stream_replays_the_script(model: FakeModel, creds: FakeCredentials) -> None:
    status, text = _post(
        model, "/anthropic/v1/messages", _anthropic_body(), {"x-api-key": creds.api_key}
    )
    assert status == 200
    events = _sse(text)
    assert [name for name, _ in events] == [
        "message_start",
        "content_block_start",
        "content_block_delta",
        "content_block_stop",
        "message_delta",
        "message_stop",
    ]
    block = json.loads(events[1][1])["content_block"]
    assert block["type"] == "tool_use"
    assert block["id"] == "toolu_1_1"
    assert block["name"] == "todo_list"
    assert json.loads(json.loads(events[2][1])["delta"]["partial_json"]) == {"todos": []}
    assert json.loads(events[4][1])["delta"]["stop_reason"] == "tool_use"

    _, text = _post(
        model, "/anthropic/v1/messages", _anthropic_body(), {"x-api-key": creds.api_key}
    )
    deltas = [json.loads(d)["delta"]["text"] for n, d in _sse(text) if n == "content_block_delta"]
    assert deltas == ["Hel", "lo"]
    stop = next(json.loads(d) for n, d in _sse(text) if n == "message_delta")
    assert stop["delta"]["stop_reason"] == "end_turn"


def test_anthropic_non_streaming_message_has_the_script_content(creds: FakeCredentials) -> None:
    with FakeModel([TOOL_TURN], creds) as m:
        body = _anthropic_body() | {"stream": False}
        status, text = _post(m, "/anthropic/v1/messages", body, {"x-api-key": creds.api_key})
        assert status == 200
        message = json.loads(text)
        assert message["content"][0]["type"] == "tool_use"
        assert message["stop_reason"] == "tool_use"


def test_hold_latch_blocks_until_released(creds: FakeCredentials) -> None:
    with FakeModel([ScriptTurn(text=("held",), hold="answer")], creds) as m:
        done = threading.Event()
        result: list[str] = []

        def call() -> None:
            result.append(
                _post(
                    m,
                    "/v1/chat/completions",
                    _openai_body(),
                    {"Authorization": f"Bearer {creds.api_key}"},
                )[1]
            )
            done.set()

        worker = threading.Thread(target=call)
        worker.start()
        m.wait_held("answer", deadline_s=10)
        assert not done.wait(0.3)
        m.release("answer")
        assert done.wait(10)
        worker.join()
        assert "held" in result[0]


def test_wait_held_fails_after_the_deadline(model: FakeModel) -> None:
    with pytest.raises(TimeoutError, match="never"):
        model.wait_held("missing", deadline_s=0.2)


def test_credential_kind_is_classified_and_no_value_is_kept(creds: FakeCredentials) -> None:
    with FakeModel([], creds) as m:
        _post(m, "/anthropic/v1/messages", _anthropic_body(), {"x-api-key": creds.api_key})
        _post(
            m,
            "/anthropic/v1/messages",
            _anthropic_body(),
            {"Authorization": f"Bearer {creds.oauth_token}"},
        )
        _post(m, "/anthropic/v1/messages", _anthropic_body(), {})
        _post(
            m, "/v1/chat/completions", _openai_body(), {"Authorization": "Bearer no-key-required"}
        )
        assert not m.failed
        _post(
            m,
            "/anthropic/v1/messages",
            _anthropic_body(),
            {"x-api-key": "sk-ant-api03-" + "x" * 40},
        )
        kinds = [r["credential"] for r in m.requests()]
        assert kinds == [
            CredentialKind.API_KEY,
            CredentialKind.OAUTH,
            CredentialKind.NONE,
            CredentialKind.NONE,
            CredentialKind.UNEXPECTED,
        ]
        assert m.failed
        dump = json.dumps(m.requests()) + repr(m) + repr(creds) + " ".join(m.failures())
        for secret in (creds.api_key, creds.oauth_token, "x" * 40):
            assert secret not in dump


@pytest.mark.parametrize(
    ("case", "headers"),
    [
        (
            "foreign bearer next to the fake key",
            {"x-api-key": "API", "Authorization": "Bearer FOREIGN"},
        ),
        (
            "foreign key next to the fake token",
            {"x-api-key": "FOREIGN", "Authorization": "Bearer OAUTH"},
        ),
        ("key and token in one request", {"x-api-key": "API", "Authorization": "Bearer OAUTH"}),
        (
            "foreign value in a further key header",
            {"api-key": "FOREIGN", "Authorization": "Bearer API"},
        ),
    ],
)
def test_every_credential_header_is_classified(
    creds: FakeCredentials, case: str, headers: dict[str, str]
) -> None:
    values = {
        "API": creds.api_key,
        "OAUTH": creds.oauth_token,
        "FOREIGN": "sk-ant-oat01-" + "y" * 40,
    }
    sent = {k: " ".join(values.get(w, w) for w in v.split(" ")) for k, v in headers.items()}
    with FakeModel([], creds) as m:
        _post(m, "/anthropic/v1/messages", _anthropic_body(), sent)
        assert [r["credential"] for r in m.requests()] == [CredentialKind.UNEXPECTED], case
        assert m.failed
        dump = json.dumps(m.requests()) + repr(m) + " ".join(m.failures())
        assert all(v not in dump for v in values.values())


def _post_raw(model: FakeModel, path: str, headers: list[tuple[str, str]]) -> None:
    conn = http.client.HTTPConnection("127.0.0.1", model.port, timeout=30)
    try:
        body = json.dumps(_anthropic_body()).encode()
        conn.putrequest("POST", path)
        for name, value in [("Content-Type", "application/json"), *headers]:
            conn.putheader(name, value)
        conn.putheader("Content-Length", str(len(body)))
        conn.endheaders(body)
        conn.getresponse().read()
    finally:
        conn.close()


@pytest.mark.parametrize(
    ("case", "path", "headers"),
    [
        (
            "a second key header",
            "/anthropic/v1/messages",
            [("x-api-key", "API"), ("x-api-key", "FOREIGN")],
        ),
        (
            "a second bearer",
            "/anthropic/v1/messages",
            [("Authorization", "Bearer OAUTH"), ("Authorization", "Bearer FOREIGN")],
        ),
        (
            "a foreign anthropic-api-key",
            "/anthropic/v1/messages",
            [("x-api-key", "API"), ("anthropic-api-key", "FOREIGN")],
        ),
        (
            "a foreign proxy credential",
            "/anthropic/v1/messages",
            [("x-api-key", "API"), ("Proxy-Authorization", "Bearer FOREIGN")],
        ),
        (
            "a foreign key in the query",
            "/anthropic/v1/messages?key=FOREIGN",
            [("x-api-key", "API")],
        ),
    ],
)
def test_repeated_headers_and_query_keys_are_classified(
    creds: FakeCredentials, case: str, path: str, headers: list[tuple[str, str]]
) -> None:
    values = {
        "API": creds.api_key,
        "OAUTH": creds.oauth_token,
        "FOREIGN": "sk-ant-api03-" + "q" * 40,
    }

    def fill(text: str) -> str:
        for word, value in values.items():
            text = text.replace(word, value)
        return text

    with FakeModel([], creds) as m:
        _post_raw(m, fill(path), [(name, fill(value)) for name, value in headers])
        assert [r["credential"] for r in m.requests()] == [CredentialKind.UNEXPECTED], case
        assert m.failed


def test_the_token_in_the_key_header_is_oauth(creds: FakeCredentials) -> None:
    # The pinned version sends every credential, the OAuth token included, as ``x-api-key`` to
    # an endpoint outside anthropic.com (``_auth_style``), and the fake endpoint is one.
    with FakeModel([], creds) as m:
        _post(m, "/anthropic/v1/messages", _anthropic_body(), {"x-api-key": creds.oauth_token})
        assert [r["credential"] for r in m.requests()] == [CredentialKind.OAUTH]
        assert not m.failed


def test_the_same_credential_in_two_headers_is_one_kind(creds: FakeCredentials) -> None:
    headers = {"x-api-key": creds.api_key, "Authorization": f"Bearer {creds.api_key}"}
    with FakeModel([], creds) as m:
        _post(m, "/anthropic/v1/messages", _anthropic_body(), headers)
        assert [r["credential"] for r in m.requests()] == [CredentialKind.API_KEY]
        assert not m.failed


def test_fake_credentials_have_the_classified_formats_and_trip_gitleaks(
    creds: FakeCredentials,
) -> None:
    assert creds.api_key.startswith("sk-ant-api03-")
    assert creds.oauth_token.startswith("sk-ant-oat01-")
    assert creds.api_key != FakeCredentials.generate().api_key
    rules = tomllib.loads((REPO / ".gitleaks.toml").read_text(encoding="utf-8"))["rules"]
    [rule] = [r for r in rules if r["id"] == "anthropic-credential-any"]
    pattern = re.compile(rule["regex"])
    assert pattern.search(creds.api_key)
    assert pattern.search(creds.oauth_token)


def test_unscripted_request_gets_the_default_answer(
    model: FakeModel, creds: FakeCredentials
) -> None:
    body = {"model": "fake-model", "messages": [{"role": "user", "content": "Title?"}]}
    status, text = _post(
        model, "/v1/chat/completions", body, {"Authorization": f"Bearer {creds.api_key}"}
    )
    assert status == 200
    assert json.loads(text)["choices"][0]["message"]["content"]
    status, text = _post(
        model,
        "/anthropic/v1/messages",
        {"model": "x", "messages": []},
        {"x-api-key": creds.api_key},
    )
    assert json.loads(text)["content"][0]["text"]
    conn = http.client.HTTPConnection("127.0.0.1", model.port, timeout=10)
    conn.request("GET", "/v1/models")
    assert json.loads(conn.getresponse().read())["data"][0]["id"] == "fake-model"
    conn.close()
    conn = http.client.HTTPConnection("127.0.0.1", model.port, timeout=10)
    conn.request("GET", "/elsewhere")
    assert conn.getresponse().status == 404
    conn.close()
    routes = [(r["route"], r["scripted"]) for r in model.requests()]
    assert routes == [
        ("openai.chat", False),
        ("anthropic.messages", False),
        ("openai.models", False),
        ("GET /elsewhere", False),
    ]


def test_exhausted_script_streams_the_default_answer(creds: FakeCredentials) -> None:
    with FakeModel([], creds) as m:
        _, text = _post(
            m, "/v1/chat/completions", _openai_body(), {"Authorization": f"Bearer {creds.api_key}"}
        )
        assert "[DONE]" in text
        assert m.requests()[0]["scripted"] is False


def test_request_shape_keeps_scenario_text_and_hashes_other_text(
    model: FakeModel, creds: FakeCredentials
) -> None:
    body = _openai_body("[Alex] Please plan the trip.\n\n[note] something else")
    body["messages"].append(
        {
            "role": "assistant",
            "content": None,
            "tool_calls": [{"id": "call_9", "function": {"name": "todo_list"}}],
        }
    )
    body["messages"].append({"role": "tool", "tool_call_id": "call_9", "content": "{}"})
    _post(model, "/v1/chat/completions", body, {"Authorization": f"Bearer {creds.api_key}"})
    [shape] = model.requests()
    assert shape["roles"] == ["system", "user", "assistant", "tool"]
    assert shape["scenario_texts"] == ["Please plan the trip."]
    assert shape["tools"] == ["todo_list"]
    assert shape["tool_call_ids"] == ["call_9"]
    assert all(re.fullmatch(r"sha256:[0-9a-f]{64}:\d+", t) for t in shape["other_texts"])
    assert "third party" not in json.dumps(shape)
    assert "something else" not in json.dumps(shape)


def test_binds_loopback_port_zero(model: FakeModel) -> None:
    assert model.port > 0
    assert model.url == f"http://127.0.0.1:{model.port}"
    assert model.openai_base_url.endswith("/v1")
    assert model.anthropic_base_url.endswith("/anthropic")


@pytest.mark.parametrize("route", ["openai", "anthropic"])
def test_paced_answer_pauses_after_every_chunk(
    route: str, creds: FakeCredentials, monkeypatch: pytest.MonkeyPatch
) -> None:
    # The pause after the last chunk lets Hermes' stream consumer draw the whole answer
    # before the end of the stream, so the last draft does not depend on timing.
    from hermes_harness import fake_model

    marker = 0.0123
    pauses: list[float] = []
    real_sleep = fake_model.time.sleep
    monkeypatch.setattr(fake_model, "CHUNK_PAUSE_S", marker)

    def sleep(seconds: float) -> None:
        if seconds == marker:
            pauses.append(seconds)
        real_sleep(seconds)

    monkeypatch.setattr(fake_model.time, "sleep", sleep)
    paced = ScriptTurn(text=("Hel", "lo"), paced=True)
    with FakeModel([paced], creds, scenario_texts=["Please plan the trip."]) as m:
        if route == "openai":
            auth = {"Authorization": f"Bearer {creds.api_key}"}
            status, _ = _post(m, "/v1/chat/completions", _openai_body(), auth)
        else:
            auth = {"x-api-key": creds.api_key, "anthropic-version": "2023-06-01"}
            status, _ = _post(m, "/anthropic/v1/messages", _anthropic_body(), auth)
    assert status == 200
    assert len(pauses) == len(paced.text)
