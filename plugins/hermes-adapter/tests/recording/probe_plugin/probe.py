# SPDX-License-Identifier: Apache-2.0
"""Probe platform plugin of the recording harness (M2-32).

The only file of the harness that imports Hermes. It uses the standard library only and the
surfaces the pinned Hermes Agent version offers to platform plugins: ``register(ctx)`` with
``ctx.register_platform`` and ``ctx.register_hook``, a subclass of ``BasePlatformAdapter``
with the pinned signatures, ``PlatformConfig.extra`` for every setting, ``build_source`` and
``handle_message`` for inbound messages. Each override posts ``{"method", "args"}`` to the
control channel of the harness, returns what the base contract expects and holds no other
logic. Commands of the harness (``inject``, ``interrupt``, ``session``, ``credential_type``,
``describe``) arrive over the same channel.

Two commands may use an internal Hermes function, and say so in their result: the credential
type (no public surface names it) and the session key of an event (documented in
``ADDING_A_PLATFORM.md`` as the seam to derive session keys).

Written for the oldest Python line the pinned version supports; no settings from
``os.environ`` (09 17.2).
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import enum
import inspect
import json
import queue
import threading
import urllib.error
import urllib.request
from typing import Any

from gateway.config import Platform, PlatformConfig
from gateway.platforms.base import BasePlatformAdapter, SendResult
from gateway.platforms.event import MessageEvent, MessageType

PLATFORM = "recording_probe"
_HOOKS = ("pre_llm_call", "on_session_start", "on_session_reset", "on_session_end")
_CHANNEL: _Channel | None = None


def _jsonable(value: Any, depth: int = 0) -> Any:
    """Convert a call argument into JSON data without losing its shape."""
    if depth > 6:
        return {"type": type(value).__name__}
    if value is None or isinstance(value, (bool, int, float, str)):
        return value
    if isinstance(value, enum.Enum):
        return {"enum": type(value).__name__, "value": _jsonable(value.value, depth + 1)}
    if isinstance(value, dict):
        return {str(k): _jsonable(v, depth + 1) for k, v in value.items()}
    if isinstance(value, (list, tuple, set, frozenset)):
        return [_jsonable(v, depth + 1) for v in value]
    if dataclasses.is_dataclass(value) and not isinstance(value, type):
        fields = {
            f.name: _jsonable(getattr(value, f.name, None), depth + 1)
            for f in dataclasses.fields(value)
        }
        return {"type": type(value).__name__, "fields": fields}
    return {"type": type(value).__name__}


def _event_summary(event: Any) -> dict[str, Any]:
    source = getattr(event, "source", None)
    return {
        "type": type(event).__name__,
        "text": getattr(event, "text", None),
        "message_id": getattr(event, "message_id", None),
        "chat_id": getattr(source, "chat_id", None),
        "thread_id": getattr(source, "thread_id", None),
        "user_id": getattr(source, "user_id", None),
    }


class _Channel:
    """Client of the harness control channel; calls are posted in call order by one thread."""

    def __init__(self, url: str) -> None:
        """Read every setting from ``PlatformConfig.extra`` and open the control channel."""
        self.url = url.rstrip("/")
        # No proxy: the channel is on 127.0.0.1, and the proxy variables point elsewhere.
        self._opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        self._calls: queue.Queue[dict[str, Any] | None] = queue.Queue()
        self._sender = threading.Thread(target=self._send_calls, name="probe-calls", daemon=True)
        self._sender.start()

    def _post(self, path: str, body: dict[str, Any]) -> None:
        request = urllib.request.Request(  # noqa: S310 - the URL is checked to be on 127.0.0.1
            self.url + path, json.dumps(body).encode("utf-8"), {"Content-Type": "application/json"}
        )
        with contextlib.suppress(urllib.error.URLError, OSError):
            self._opener.open(request, timeout=30).read()

    def _send_calls(self) -> None:
        while True:
            item = self._calls.get()
            if item is None:
                return
            self._post("/call", item)

    def call(self, method: str, args: dict[str, Any]) -> None:
        self._calls.put({"method": method, "args": _jsonable(args)})

    def next_command(self) -> dict[str, Any] | None:
        try:
            with self._opener.open(self.url + "/command", timeout=30) as response:
                if response.status == 204:
                    return None
                return json.loads(response.read().decode("utf-8"))
        except (urllib.error.URLError, OSError, ValueError):
            return None

    def result(self, command_id: str, result: Any) -> None:
        self._post("/result", {"id": command_id, "result": _jsonable(result)})


def _hook(name: str):
    def callback(**kwargs: Any) -> None:
        if _CHANNEL is None or str(kwargs.get("platform") or "") != PLATFORM:
            return None
        _CHANNEL.call(
            "hook:" + name,
            {
                "session_id": kwargs.get("session_id"),
                "is_first_turn": kwargs.get("is_first_turn"),
                "user_message": kwargs.get("user_message"),
                "kwargs": sorted(kwargs),
            },
        )
        return None

    return callback


class RecordingProbe(BasePlatformAdapter):
    """Platform adapter that records every call of the pinned Hermes version."""

    def __init__(self, config: PlatformConfig) -> None:
        """Read every setting from ``PlatformConfig.extra`` and open the control channel."""
        super().__init__(config, Platform(PLATFORM))
        global _CHANNEL  # noqa: PLW0603 - hooks registered in register(ctx) post through it
        extra = dict(getattr(config, "extra", None) or {})
        self._extra = extra
        self._channel = _Channel(str(extra["control_url"]))
        _CHANNEL = self._channel
        self._poll = float(extra.get("command_poll_s", 0.05))
        self._draft = bool(extra.get("supports_draft_streaming", False))
        self.supports_status_text = bool(extra.get("supports_status_text", False))
        self._own_policy = bool(extra.get("enforces_own_access_policy", False))
        self._upstream = bool(extra.get("authorization_is_upstream", False))
        self._sent = 0
        self._loop_task: asyncio.Task[None] | None = None
        self._events: dict[str, MessageEvent] = {}
        self._confirm: tuple | None = None

    # ---- recording helpers -----------------------------------------------------------
    def _record(self, method: str, **args: Any) -> None:
        self._channel.call(method, args)

    def _next_id(self) -> str:
        self._sent += 1
        return "p" + str(self._sent)

    @property
    def enforces_own_access_policy(self) -> bool:  # type: ignore[override]
        """Declared own access policy (recorded on every read by the gateway)."""
        self._record("read:enforces_own_access_policy", value=self._own_policy)
        return self._own_policy

    @property
    def authorization_is_upstream(self) -> bool:  # type: ignore[override]
        """Declared upstream authorization (Hermes v0.22; recorded on every read)."""
        self._record("read:authorization_is_upstream", value=self._upstream)
        return self._upstream

    # ---- lifecycle -------------------------------------------------------------------
    async def connect(self, *, is_reconnect: bool = False) -> bool:
        """Declared own access policy (recorded on every read by the gateway)."""
        self._record(
            "connect",
            is_reconnect=is_reconnect,
            extra_keys=sorted(k for k in self._extra if k != "control_url"),
        )
        self._mark_connected()
        self._loop_task = asyncio.get_running_loop().create_task(self._command_loop())
        return True

    async def disconnect(self) -> None:
        """Record the call and stop the command loop."""
        self._record("disconnect")
        if self._loop_task is not None:
            self._loop_task.cancel()
        self._mark_disconnected()

    # ---- outbound --------------------------------------------------------------------
    async def send(
        self,
        chat_id: str,
        content: str,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """Record a send; return a sequential message ID."""
        message_id = self._next_id()
        self._record(
            "send",
            chat_id=chat_id,
            content=content,
            reply_to=reply_to,
            metadata=metadata,
            message_id=message_id,
        )
        return SendResult(success=True, message_id=message_id)

    async def edit_message(
        self, chat_id: str, message_id: str, content: str, *, finalize: bool = False
    ) -> SendResult:
        """Record an edit of an earlier message."""
        self._record(
            "edit_message",
            chat_id=chat_id,
            message_id=message_id,
            content=content,
            finalize=finalize,
        )
        return SendResult(success=True, message_id=message_id)

    def supports_draft_streaming(
        self,
        chat_type: str | None = None,
        metadata: dict[str, Any] | None = None,
        chat_id: str | None = None,
    ) -> bool:
        """Answer the draft streaming flag of the scenario."""
        return self._draft

    async def send_draft(
        self, chat_id: str, draft_id: int, content: str, metadata: dict[str, Any] | None = None
    ) -> SendResult:
        """Record a streaming draft."""
        self._record(
            "send_draft", chat_id=chat_id, draft_id=draft_id, content=content, metadata=metadata
        )
        return SendResult(success=True)

    def set_status_text(self, chat_id: str, text: str | None) -> None:
        """Record a status text (``None`` clears it)."""
        self._record("set_status_text", chat_id=chat_id, text=text)

    def format_tool_event(
        self, event: Any, *, mode: str = "all", preview_max_len: int = 40
    ) -> str | None:
        """Record a tool event; ``None`` asks Hermes to send no progress message."""
        self._record("format_tool_event", event=event, mode=mode, preview_max_len=preview_max_len)
        return None

    def render_message_event(self, event: Any, sink: Any) -> None:
        """Record a stream event and keep the standard rendering."""
        self._record("render_message_event", event=event)
        super().render_message_event(event, sink)

    async def send_typing(self, chat_id: str, metadata=None) -> None:
        """Record a typing indicator."""
        self._record("send_typing", chat_id=chat_id)

    async def stop_typing(self, chat_id: str) -> None:
        """Record the end of a typing indicator."""
        self._record("stop_typing", chat_id=chat_id)

    async def send_image(
        self,
        chat_id: str,
        image_url: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """Record an image send."""
        self._record("send_image", chat_id=chat_id, image_url=image_url, caption=caption)
        return SendResult(success=True, message_id=self._next_id())

    async def send_document(
        self,
        chat_id: str,
        file_path: str,
        caption: str | None = None,
        file_name: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs,
    ) -> SendResult:
        """Record a document send."""
        self._record("send_document", chat_id=chat_id, file_path=file_path, caption=caption)
        return SendResult(success=True, message_id=self._next_id())

    async def send_voice(
        self,
        chat_id: str,
        audio_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs,
    ) -> SendResult:
        """Record a voice send."""
        self._record("send_voice", chat_id=chat_id, audio_path=audio_path, caption=caption)
        return SendResult(success=True, message_id=self._next_id())

    async def send_video(
        self,
        chat_id: str,
        video_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs,
    ) -> SendResult:
        """Record a video send."""
        self._record("send_video", chat_id=chat_id, video_path=video_path, caption=caption)
        return SendResult(success=True, message_id=self._next_id())

    async def send_clarify(
        self,
        chat_id: str,
        question: str,
        choices: list | None,
        clarify_id: str,
        session_key: str,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """Record a clarify prompt."""
        self._record(
            "send_clarify",
            chat_id=chat_id,
            question=question,
            choices=choices,
            clarify_id=clarify_id,
        )
        return SendResult(success=True, message_id=self._next_id())

    async def _send_exec_approval_prompt(self, prompt: Any) -> SendResult:
        """Record an approval prompt (documented override point)."""
        self._record("_send_exec_approval_prompt", prompt=prompt)
        return SendResult(success=True, message_id=self._next_id())

    async def send_slash_confirm(
        self,
        chat_id: str,
        title: str,
        message: str,
        session_key: str,
        confirm_id: str,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """Record a slash command confirmation and keep it for ``confirm``."""
        self._record(
            "send_slash_confirm",
            chat_id=chat_id,
            title=title,
            message=message,
            confirm_id=confirm_id,
        )
        self._confirm = (session_key, confirm_id)
        return SendResult(success=True, message_id=self._next_id())

    async def interrupt_session_activity(
        self, session_key: str, chat_id: str, metadata=None
    ) -> None:
        """Record the interruption hook and keep the base behaviour."""
        self._record("interrupt_session_activity", chat_id=chat_id)
        await super().interrupt_session_activity(session_key, chat_id, metadata)

    async def on_processing_start(self, event: MessageEvent) -> None:
        """Record the start of processing."""
        self._record("on_processing_start", event=_event_summary(event))

    async def on_processing_complete(self, event: MessageEvent, outcome: Any) -> None:
        """Record the end of processing with its outcome."""
        self._record("on_processing_complete", event=_event_summary(event), outcome=outcome)

    async def get_chat_info(self, chat_id: str) -> dict[str, Any]:
        """Record the lookup; answer a thread."""
        self._record("get_chat_info", chat_id=chat_id)
        return {"name": chat_id, "type": "thread", "chat_id": chat_id}

    # ---- commands of the harness -------------------------------------------------------
    async def _command_loop(self) -> None:
        while True:
            command = await asyncio.to_thread(self._channel.next_command)
            if command is None:
                await asyncio.sleep(self._poll)
                continue
            try:
                result = await self._run(str(command.get("name")), dict(command.get("args") or {}))
            except Exception as exc:  # the harness fails the step on an error result
                result = {"error": type(exc).__name__}
            self._channel.result(str(command.get("id")), result)

    def _event(self, args: dict[str, Any]) -> MessageEvent:
        source = self.build_source(
            chat_id=args["chat_id"],
            chat_type="thread",
            thread_id=args["thread_id"],
            user_id=args["user_id"],
            user_name=args["user_name"],
            message_id=args["message_id"],
        )
        event = MessageEvent(
            text=args["text"],
            message_type=MessageType.TEXT,
            source=source,
            message_id=args["message_id"],
        )
        self._events[args["thread_id"]] = event
        return event

    async def _run(self, name: str, args: dict[str, Any]) -> Any:
        if name == "inject":
            await self.handle_message(self._event(args))
            return {"ok": True}
        if name == "interrupt":
            return await self._interrupt(args)
        if name == "session":
            await self.handle_message(self._event(args))
            return {"ok": True, "way": "command via handle_message"}
        if name == "confirm":
            # The resolver ADDING_A_PLATFORM.md names for send_slash_confirm buttons.
            from tools.slash_confirm import resolve  # noqa: PLC0415 - only when asked

            if self._confirm is None:
                return {"error": "no pending confirmation"}
            session_key, confirm_id = self._confirm
            self._confirm = None
            output = await resolve(session_key, confirm_id, str(args.get("choice", "once")))
            return {
                "ok": True,
                "resolved": output is not None,
                "way": "tools.slash_confirm.resolve(session_key, confirm_id, choice)",
            }
        if name == "credential_type":
            return _credential_type()
        if name == "describe":
            return _describe(type(self), list(args.get("surfaces") or []))
        if name == "ready":
            # Harness timing only: inbound waits in a queue until the startup restore of the
            # gateway finished. The flag is internal; it decides when a scenario starts, no fact.
            runner = getattr(self, "gateway_runner", None)
            in_progress = getattr(runner, "_startup_restore_in_progress", None)
            return {"ready": runner is not None and in_progress is False}
        return {"error": "unknown command " + name}

    async def _interrupt(self, args: dict[str, Any]) -> Any:
        if args.get("way") == "method":
            # cancel_session_processing: public name on BasePlatformAdapter; the key comes from
            # the documented seam _event_session_key (ADDING_A_PLATFORM.md "Key patterns").
            event = self._events[args["thread_id"]]
            await self.cancel_session_processing(self._event_session_key(event))
            return {"ok": True, "way": "BasePlatformAdapter.cancel_session_processing"}
        await self.handle_message(self._event(args))
        return {"ok": True, "way": "command via handle_message"}


def _credential_type() -> dict[str, Any]:
    """Kind of the credential the next turn uses, from the function Hermes itself uses.

    No public surface exists in the pinned version; the internal resolver is named in the
    result. The credential value stays in this frame and is never posted.
    """
    surface = (
        "hermes_cli.runtime_provider.resolve_runtime_provider() and "
        "agent.anthropic_credentials.anthropic_route_is_oauth(base_url, credential, *, provider)"
    )
    try:
        from agent.anthropic_credentials import anthropic_route_is_oauth  # noqa: PLC0415
        from hermes_cli.runtime_provider import resolve_runtime_provider  # noqa: PLC0415
    except ImportError:
        return {
            "kind": "unknown",
            "surface": surface,
            "public": False,
            "resolution_error": "ImportError",
        }
    try:
        runtime = resolve_runtime_provider()
    except Exception as exc:
        return {
            "kind": "none",
            "surface": surface,
            "public": False,
            "resolution_error": type(exc).__name__,
        }
    credential = runtime.get("api_key")
    try:
        if anthropic_route_is_oauth(
            runtime.get("base_url"), credential, provider=runtime.get("provider")
        ):
            kind = "oauth"
        elif isinstance(credential, str) and credential and credential != "no-key-required":
            kind = "api_key"
        else:
            kind = "none"
    finally:
        credential = None
    return {
        "kind": kind,
        "surface": surface,
        "public": False,
        "provider": runtime.get("provider"),
        "source": runtime.get("source"),
    }


def _describe(adapter_cls: type, surfaces: list[str]) -> dict[str, Any]:
    """Presence and parameter names of each named surface in the pinned version."""
    import gateway.config as config_module  # noqa: PLC0415 - only for describe
    import gateway.platform_registry as registry_module  # noqa: PLC0415
    from hermes_cli.plugins import PluginContext  # noqa: PLC0415

    found: dict[str, Any] = {}
    owners = {
        "ctx": PluginContext,
        "base": BasePlatformAdapter,
        "entry": getattr(registry_module, "PlatformEntry", None),
        "config": getattr(config_module, "PlatformConfig", None),
    }
    for surface in surfaces:
        owner_name, _, attr = surface.partition(".")
        owner = owners.get(owner_name)
        if owner is None:
            found[surface] = {"present": False}
            continue
        if owner_name in ("entry", "config") and dataclasses.is_dataclass(owner):
            names = [f.name for f in dataclasses.fields(owner)]
            found[surface] = {"present": attr in names, "kind": "field"}
            continue
        member = inspect.getattr_static(owner, attr, None)
        if member is None:
            found[surface] = {"present": False}
            continue
        target = (
            member.fget if isinstance(member, property) else getattr(member, "__func__", member)
        )
        if callable(target):
            try:
                params = [p for p in inspect.signature(target).parameters if p != "self"]
            except (TypeError, ValueError):
                params = []
            kind = "property" if isinstance(member, property) else "method"
            found[surface] = {"present": True, "kind": kind, "params": params}
        else:
            found[surface] = {"present": True, "kind": "attribute", "value": _jsonable(member)}
    return found


def register(ctx: Any) -> None:
    """Plugin entry point: register the probe platform and the session hooks."""
    ctx.register_platform(
        name=PLATFORM,
        label="Recording probe",
        adapter_factory=RecordingProbe,
        check_fn=lambda: True,
        allowed_users_env="RECORDING_PROBE_ALLOWED_USERS",
        allow_all_env="RECORDING_PROBE_ALLOW_ALL_USERS",
    )
    for name in _HOOKS:
        ctx.register_hook(name, _hook(name))
