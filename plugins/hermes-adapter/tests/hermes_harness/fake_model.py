# SPDX-License-Identifier: Apache-2.0
"""The local fake model endpoint and the fake credentials of a run.

:class:`FakeModel` serves on ``127.0.0.1`` port 0 an OpenAI-compatible route (chat
completions, streaming and non-streaming, and the model list) and an Anthropic route
(messages, streaming events from message start to message stop, tool use blocks). Each
scripted request replays the next :class:`ScriptTurn`: tool calls or text in fixed chunks
with fixed usage numbers, optionally held on a named latch until :meth:`FakeModel.release`.
A request the script does not cover (titles, compression, metadata probes) gets a fixed
short answer and is recorded as unscripted.

Each request is kept only as its shape: route, model name, roles, the scenario texts it
contains (verbatim), tool names and call IDs, ``sha256:<hex>:<length>`` for every other text,
and the :class:`CredentialKind` of the presented credential. A credential value is compared
with the fake credentials of the run and dropped at once; an unknown value is
``unexpected`` and fails the run.

Example:
    >>> creds = FakeCredentials.generate()
    >>> with FakeModel([ScriptTurn(text=("Hello",))], creds, scenario_texts=["Hi"]) as model:
    ...     config = {"base_url": model.openai_base_url}
"""

from __future__ import annotations

import enum
import hashlib
import json
import secrets
import threading
import time
import urllib.parse
from collections.abc import Iterator, Sequence
from dataclasses import dataclass, field
from typing import Any, Final

from hermes_harness._server import LoopbackServer, QuietHandler
from hermes_harness.limits import CHUNK_PAUSE_S, STEP_DEADLINE_S

#: The model name every configured provider and auxiliary task uses.
MODEL_NAME = "fake-model"
#: Context and output size the configuration and the metadata catalogue name for the fake model,
#: so the pinned version never probes a real metadata service for them.
CONTEXT_LENGTH = 100000
OUTPUT_LENGTH = 4096
#: Fixed usage numbers of every answer.
USAGE = {"input": 11, "output": 7}
#: Credential placeholders that Hermes itself sends to keyless local endpoints.
KEYLESS_PLACEHOLDERS = frozenset({"no-key-required"})
#: The answer to a request the script does not cover.
DEFAULT_ANSWER = "Noted."
#: Path of the model metadata catalogue the pinned version reads (``models_dev.url``).
MODELS_DEV_PATH = "/models-dev/api.json"
#: The catalogue served there: the fake model under the fake provider, nothing else.
MODELS_DEV_CATALOGUE = {
    "fake": {
        "id": "fake",
        "name": "Fake",
        "models": {
            MODEL_NAME: {
                "id": MODEL_NAME,
                "name": MODEL_NAME,
                "limit": {"context": CONTEXT_LENGTH, "output": OUTPUT_LENGTH},
            }
        },
    }
}


class CredentialKind(enum.StrEnum):
    """Which credential a request presented, never its value."""

    API_KEY = "api_key"
    OAUTH = "oauth"
    NONE = "none"
    UNEXPECTED = "unexpected"


@dataclass(frozen=True)
class FakeCredentials:
    """A fake API key and a fake OAuth token, generated per run.

    The formats are the ones the pinned Hermes version uses to classify a credential
    (``sk-ant-api`` for a Console key, another ``sk-ant-`` type for OAuth); the content is
    random. They live only in memory and in the throwaway homes.
    """

    api_key: str = field(repr=False)
    oauth_token: str = field(repr=False)

    @classmethod
    def generate(cls) -> FakeCredentials:
        """Create a new pair with random content."""
        return cls(
            api_key="sk-ant-api03-" + secrets.token_urlsafe(48),
            oauth_token="sk-ant-oat01-" + secrets.token_urlsafe(48),
        )

    def classify(self, value: str | None) -> CredentialKind:
        """Map a presented value to its kind; the value itself is not kept."""
        if not value or value in KEYLESS_PLACEHOLDERS:
            return CredentialKind.NONE
        if secrets.compare_digest(value, self.api_key):
            return CredentialKind.API_KEY
        if secrets.compare_digest(value, self.oauth_token):
            return CredentialKind.OAUTH
        return CredentialKind.UNEXPECTED


@dataclass(frozen=True)
class ScriptTurn:
    """One scripted model answer.

    Attributes:
        text: Text chunks streamed in this order (empty for a tool turn).
        tool_calls: Tool calls ``{"name": ..., "arguments": {...}}`` of this answer.
        hold: Name of a latch; the answer waits for :meth:`FakeModel.release` first.
        paced: Pause :data:`~hermes_harness.limits.CHUNK_PAUSE_S` after every text chunk, so a
            streaming consumer sees the answer grow and draws the whole answer before the
            stream ends.
    """

    text: tuple[str, ...] = ()
    tool_calls: tuple[dict[str, Any], ...] = ()
    hold: str | None = None
    paced: bool = False

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> ScriptTurn:
        """Build a turn from its scenario JSON form."""
        return cls(
            text=tuple(data.get("text", ())),
            tool_calls=tuple(data.get("tool_calls", ())),
            hold=data.get("hold"),
            paced=bool(data.get("paced", False)),
        )


def digest_text(text: str) -> str:
    """``sha256:<hex>:<length>`` of a text."""
    return f"sha256:{hashlib.sha256(text.encode('utf-8')).hexdigest()}:{len(text)}"


def _texts_of(content: Any) -> Iterator[str]:
    if isinstance(content, str):
        yield content
    elif isinstance(content, list):
        for part in content:
            if isinstance(part, dict):
                if isinstance(part.get("text"), str):
                    yield part["text"]
                if "content" in part:
                    yield from _texts_of(part["content"])


class _Recorder:
    def __init__(self, scenario_texts: Sequence[str]) -> None:
        self.scenario_texts = [t for t in scenario_texts if t]
        self.texts: list[str] = []
        self.other: list[str] = []
        self.call_ids: list[str] = []

    def text(self, value: str) -> None:
        found = [t for t in self.scenario_texts if t in value]
        self.texts += [t for t in found if t not in self.texts]
        if value not in self.scenario_texts:
            self.other.append(digest_text(value))

    def call_id(self, value: Any) -> None:
        if isinstance(value, str) and value and value not in self.call_ids:
            self.call_ids.append(value)


def _shape(route: str, body: dict[str, Any], scenario_texts: Sequence[str]) -> dict[str, Any]:
    rec = _Recorder(scenario_texts)
    roles: list[str] = []
    system = body.get("system")
    if system is not None:
        roles.append("system")
        for text in _texts_of(system):
            rec.text(text)
    for message in body.get("messages") or []:
        if not isinstance(message, dict):
            continue
        roles.append(str(message.get("role")))
        for text in _texts_of(message.get("content")):
            rec.text(text)
        for call in message.get("tool_calls") or []:
            rec.call_id(call.get("id") if isinstance(call, dict) else None)
        rec.call_id(message.get("tool_call_id"))
        if isinstance(message.get("content"), list):
            for part in message["content"]:
                if isinstance(part, dict):
                    rec.call_id(part.get("id") if part.get("type") == "tool_use" else None)
                    rec.call_id(part.get("tool_use_id"))
    tools = []
    for tool in body.get("tools") or []:
        if isinstance(tool, dict):
            name = (tool.get("function") or {}).get("name") or tool.get("name")
            if isinstance(name, str):
                tools.append(name)
    return {
        "route": route,
        "model": body.get("model"),
        "stream": bool(body.get("stream")),
        "roles": roles,
        "scenario_texts": rec.texts,
        "other_texts": rec.other,
        "tools": tools,
        "tool_call_ids": rec.call_ids,
    }


#: Request headers that carry an API key at one of the providers the pinned version speaks to.
KEY_HEADERS: Final = ("x-api-key", "api-key", "anthropic-api-key", "x-goog-api-key")
#: Headers that carry a credential, optionally with an auth scheme such as ``Bearer``.
SCHEME_HEADERS: Final = ("Authorization", "Proxy-Authorization")
#: Query parameters that carry a credential at one of those providers.
KEY_PARAMS: Final = ("key", "api_key", "apikey", "access_token", "token")


def _without_scheme(value: str) -> str:
    scheme, _, rest = value.partition(" ")
    return rest.strip() if rest and scheme.lower() in {"bearer", "basic", "token"} else value


class _Handler(QuietHandler):
    @property
    def model(self) -> FakeModel:
        owner: FakeModel = self.server.owner  # type: ignore[attr-defined]
        return owner

    def _credential_kind(self) -> CredentialKind:
        """The one kind of credential this request carries, judged over every header.

        Every value of every credential header and credential query parameter counts: a
        foreign value in any of them or two different credentials in one request is
        ``unexpected``, so a real credential cannot ride along next to a fake one. Which header
        carries a fake credential is not judged: the pinned model client sends every credential,
        the OAuth token included, as ``x-api-key`` to an endpoint outside anthropic.com, while
        its context length probe sends the OAuth token as a bearer token.
        """
        creds = self.model.credentials
        values = [v for h in KEY_HEADERS for v in self.headers.get_all(h) or ()]
        values += [
            _without_scheme(v) for h in SCHEME_HEADERS for v in self.headers.get_all(h) or ()
        ]
        query = urllib.parse.parse_qs(urllib.parse.urlsplit(self.path).query)
        values += [v for name in KEY_PARAMS for v in query.get(name, ())]
        found = {creds.classify(v) for v in values} - {CredentialKind.NONE}
        if CredentialKind.UNEXPECTED in found or len(found) > 1:
            return CredentialKind.UNEXPECTED
        return found.pop() if found else CredentialKind.NONE

    def do_GET(self) -> None:  # noqa: N802 - http.server dispatch name
        """Model list and metadata probes."""
        path = self.path.split("?", 1)[0]
        kind = self._credential_kind()
        if path == MODELS_DEV_PATH:
            self.model.record({"route": "models_dev", "credential": kind, "scripted": False})
            self.reply(200, json.dumps(MODELS_DEV_CATALOGUE).encode(), "application/json")
            return
        if path.endswith("/models"):
            self.model.record({"route": "openai.models", "credential": kind, "scripted": False})
            body = {
                "object": "list",
                "data": [{"id": MODEL_NAME, "object": "model", "owned_by": "fake"}],
            }
            self.reply(200, json.dumps(body).encode(), "application/json")
            return
        self.model.record({"route": f"GET {path}", "credential": kind, "scripted": False})
        self.reply(404, b"not served by the fake model\n")

    def do_POST(self) -> None:  # noqa: N802 - http.server dispatch name
        """Chat completions and messages."""
        path = self.path.split("?", 1)[0]
        raw = self.read_body()
        try:
            body = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            body = {}
        if not isinstance(body, dict):
            body = {}
        if path.endswith("/chat/completions"):
            route = "openai.chat"
        elif path.endswith("/messages"):
            route = "anthropic.messages"
        else:
            kind = self._credential_kind()
            self.model.record({"route": f"POST {path}", "credential": kind, "scripted": False})
            self.reply(404, b"not served by the fake model\n")
            return
        shape = _shape(route, body, self.model.scenario_texts)
        shape["credential"] = self._credential_kind()
        turn = self.model.next_turn(shape)
        if turn is not None and turn.hold:
            self.model.hold(turn.hold)
        answer = turn or ScriptTurn(text=(DEFAULT_ANSWER,))
        try:
            if route == "openai.chat":
                self._openai(answer, shape, stream=shape["stream"])
            else:
                self._anthropic(answer, shape, stream=shape["stream"])
            shape["completed"] = True
        except (BrokenPipeError, ConnectionResetError):
            shape["completed"] = False
        self.close_connection = True

    def _start_stream(self) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()

    def _event(self, data: str, name: str | None = None) -> None:
        head = f"event: {name}\n" if name else ""
        self.wfile.write(f"{head}data: {data}\n\n".encode())
        self.wfile.flush()

    @staticmethod
    def _pace(turn: ScriptTurn) -> None:
        # A pause after every chunk, the last one included: the stream consumer of Hermes
        # draws only the deltas it drains before the end of the stream.
        if turn.paced:
            time.sleep(CHUNK_PAUSE_S)

    def _openai(self, turn: ScriptTurn, shape: dict[str, Any], *, stream: bool) -> None:
        number = shape.get("turn") or 0
        calls = [
            {
                "index": i,
                "id": f"call_{number}_{i + 1}",
                "type": "function",
                "function": {"name": c["name"], "arguments": json.dumps(c.get("arguments", {}))},
            }
            for i, c in enumerate(turn.tool_calls)
        ]
        usage = {
            "prompt_tokens": USAGE["input"],
            "completion_tokens": USAGE["output"],
            "total_tokens": USAGE["input"] + USAGE["output"],
        }
        finish = "tool_calls" if calls else "stop"
        base = {
            "id": f"chatcmpl-{number}",
            "object": "chat.completion.chunk",
            "created": 0,
            "model": MODEL_NAME,
        }
        if not stream:
            message: dict[str, Any] = {"role": "assistant", "content": "".join(turn.text) or None}
            if calls:
                message["tool_calls"] = [
                    {k: v for k, v in c.items() if k != "index"} for c in calls
                ]
            body = base | {
                "object": "chat.completion",
                "choices": [{"index": 0, "message": message, "finish_reason": finish}],
                "usage": usage,
            }
            self.reply(200, json.dumps(body).encode(), "application/json")
            return
        self._start_stream()
        if calls:
            delta = {"role": "assistant", "tool_calls": calls}
            self._event(
                json.dumps(
                    base | {"choices": [{"index": 0, "delta": delta, "finish_reason": None}]}
                )
            )
        for piece in turn.text:
            delta = {"role": "assistant", "content": piece}
            self._event(
                json.dumps(
                    base | {"choices": [{"index": 0, "delta": delta, "finish_reason": None}]}
                )
            )
            self._pace(turn)
        end = base | {
            "choices": [{"index": 0, "delta": {}, "finish_reason": finish}],
            "usage": usage,
        }
        self._event(json.dumps(end))
        self._event("[DONE]")

    def _anthropic(self, turn: ScriptTurn, shape: dict[str, Any], *, stream: bool) -> None:
        number = shape.get("turn") or 0
        stop = "tool_use" if turn.tool_calls else "end_turn"
        usage = {"input_tokens": USAGE["input"], "output_tokens": USAGE["output"]}
        blocks: list[dict[str, Any]] = []
        if turn.text:
            blocks.append({"type": "text", "text": "".join(turn.text)})
        for i, call in enumerate(turn.tool_calls):
            blocks.append(
                {
                    "type": "tool_use",
                    "id": f"toolu_{number}_{i + 1}",
                    "name": call["name"],
                    "input": call.get("arguments", {}),
                }
            )
        message = {
            "id": f"msg_{number}",
            "type": "message",
            "role": "assistant",
            "model": MODEL_NAME,
            "content": blocks,
            "stop_reason": stop,
            "stop_sequence": None,
            "usage": usage,
        }
        if not stream:
            self.reply(200, json.dumps(message).encode(), "application/json")
            return
        self._start_stream()
        start = message | {
            "content": [],
            "stop_reason": None,
            "usage": usage | {"output_tokens": 0},
        }
        self._event(json.dumps({"type": "message_start", "message": start}), "message_start")
        index = 0
        if turn.text:
            block = {
                "type": "content_block_start",
                "index": index,
                "content_block": {"type": "text", "text": ""},
            }
            self._event(json.dumps(block), "content_block_start")
            for piece in turn.text:
                delta = {
                    "type": "content_block_delta",
                    "index": index,
                    "delta": {"type": "text_delta", "text": piece},
                }
                self._event(json.dumps(delta), "content_block_delta")
                self._pace(turn)
            self._event(
                json.dumps({"type": "content_block_stop", "index": index}), "content_block_stop"
            )
            index += 1
        for block in blocks[index:] if turn.text else blocks:
            if block["type"] != "tool_use":
                continue
            head = {
                "type": "content_block_start",
                "index": index,
                "content_block": {
                    "type": "tool_use",
                    "id": block["id"],
                    "name": block["name"],
                    "input": {},
                },
            }
            self._event(json.dumps(head), "content_block_start")
            delta = {
                "type": "content_block_delta",
                "index": index,
                "delta": {"type": "input_json_delta", "partial_json": json.dumps(block["input"])},
            }
            self._event(json.dumps(delta), "content_block_delta")
            self._event(
                json.dumps({"type": "content_block_stop", "index": index}), "content_block_stop"
            )
            index += 1
        end = {
            "type": "message_delta",
            "delta": {"stop_reason": stop, "stop_sequence": None},
            "usage": {"output_tokens": USAGE["output"]},
        }
        self._event(json.dumps(end), "message_delta")
        self._event(json.dumps({"type": "message_stop"}), "message_stop")


class FakeModel(LoopbackServer):
    """OpenAI-compatible and Anthropic fake endpoint on ``127.0.0.1`` port 0."""

    def __init__(
        self,
        script: Sequence[ScriptTurn],
        credentials: FakeCredentials,
        *,
        scenario_texts: Sequence[str] = (),
    ) -> None:
        """Bind the endpoint.

        Args:
            script: The answers of the scripted requests, in order.
            credentials: The fake credentials of the run.
            scenario_texts: Texts the scenario injected; kept verbatim in request shapes.
        """
        super().__init__(_Handler)
        self.credentials = credentials
        self.scenario_texts = list(scenario_texts)
        self._script = list(script)
        self._lock = threading.Lock()
        self._requests: list[dict[str, Any]] = []
        self._failures: list[str] = []
        self._turns = 0
        self._held: dict[str, threading.Event] = {}
        self._released: dict[str, threading.Event] = {}

    @property
    def openai_base_url(self) -> str:
        """Base URL of the OpenAI-compatible route."""
        return f"{self.url}/v1"

    @property
    def models_dev_url(self) -> str:
        """URL of the model metadata catalogue (instead of the real metadata service)."""
        return f"{self.url}{MODELS_DEV_PATH}"

    @property
    def anthropic_base_url(self) -> str:
        """Base URL of the Anthropic route (Hermes appends ``/v1/messages``)."""
        return f"{self.url}/anthropic"

    def _latch(self, name: str) -> tuple[threading.Event, threading.Event]:
        with self._lock:
            return (
                self._held.setdefault(name, threading.Event()),
                self._released.setdefault(name, threading.Event()),
            )

    def record(self, shape: dict[str, Any]) -> None:
        """Keep a request shape; an unexpected credential marks the run failed."""
        with self._lock:
            self._requests.append(shape)
            if shape.get("credential") == CredentialKind.UNEXPECTED:
                self._failures.append(f"unexpected credential on {shape.get('route')}")

    def next_turn(self, shape: dict[str, Any]) -> ScriptTurn | None:
        """Assign the next script turn to a request that offers tools; record the shape."""
        turn: ScriptTurn | None = None
        with self._lock:
            if shape["tools"] and self._turns < len(self._script):
                turn = self._script[self._turns]
                self._turns += 1
                shape["turn"] = self._turns
            else:
                shape["turn"] = None
            shape["scripted"] = turn is not None
        self.record(shape)
        return turn

    def hold(self, name: str) -> None:
        """Block the calling request until the latch is released (or the step deadline)."""
        held, released = self._latch(name)
        held.set()
        if not released.wait(STEP_DEADLINE_S):
            with self._lock:
                self._failures.append(f"latch {name} was never released")

    def wait_held(self, name: str, *, deadline_s: float = STEP_DEADLINE_S) -> None:
        """Wait until a request waits on the latch.

        Raises:
            TimeoutError: No request reached the latch before the deadline.
        """
        held, _ = self._latch(name)
        if not held.wait(deadline_s):
            raise TimeoutError(f"latch {name} was never reached within the step deadline")

    def release(self, name: str) -> None:
        """Let the request waiting on the latch continue."""
        self._latch(name)[1].set()

    def requests(self) -> list[dict[str, Any]]:
        """All request shapes in arrival order (copies)."""
        with self._lock:
            return [dict(r) for r in self._requests]

    def failures(self) -> list[str]:
        """Reasons that fail the run (unexpected credential, latch never released)."""
        with self._lock:
            return list(self._failures)

    @property
    def failed(self) -> bool:
        """Whether a failure was recorded."""
        return bool(self.failures())

    def close(self) -> None:
        """Release every latch, then stop serving."""
        with self._lock:
            events = list(self._released.values())
        for event in events:
            event.set()
        time.sleep(0)
        super().close()

    def __repr__(self) -> str:
        """Name the port and counts only."""
        return f"FakeModel(port={self.port}, requests={len(self._requests)})"
