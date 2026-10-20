// SPDX-License-Identifier: Apache-2.0

/// The state machine of one terminal tab (docs/spec/15-client-architektur.md 23.2, brief
/// M4-35 Design note 1).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart'
    show
        ApiProblem,
        Host,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionOpen;
import 'package:auth/auth.dart' show Clock, StepUpCancelled;
import 'package:bothy_noise/bothy_noise.dart';
import 'package:cryptography/cryptography.dart' show SimpleKeyPair;
import 'package:domain/domain.dart' show AppConfig, AppConfigValues, HostId;
import 'package:flutter/foundation.dart';
import 'package:flutter/scheduler.dart';
import 'package:terminal_ui/src/controller/close_reactions.dart';
import 'package:terminal_ui/src/controller/frame_pump.dart';
import 'package:terminal_ui/src/controller/session_state.dart';
import 'package:terminal_ui/src/controller/terminal_prompts.dart';
import 'package:terminal_ui/src/pinning/pin_check.dart';
import 'package:terminal_ui/src/ports.dart';
import 'package:terminal_ui/src/render/terminal_screen.dart';

/// The protocol version of the stream this client speaks (asyncapi `x-protocol-versions`).
const int terminalStreamProtocolVersion = 1;

/// Creates the ephemeral X25519 key of one handshake; tests pass their own to watch it.
typedef EphemeralKeyFactory = Future<SimpleKeyPair> Function();

/// Runs one terminal tab: opening, the stream, input, size changes, detaching, reattaching
/// and ending (15 23.2), with the pin check of 15 23.3 and the warning of 24 8.4.
///
/// The controller is the only owner of the ephemeral private key: it creates the key right
/// before the request, hands it to `NkInitiator` and destroys it (zeroes its bytes) right after
/// message 2 (R-CL-15); no other object receives it. Terminal content never reaches a log, an
/// exception text or a metric (R-CL-17): errors carry codes only.
///
/// ```dart
/// final c = TerminalController(api: api, transport: transport, pins: pins,
///     stepUp: session.stepUp.confirm, links: opener, clock: const SystemClock(),
///     prompts: DialogTerminalPrompts(() => navigatorKey.currentContext),
///     selfUserId: me.id, config: appConfig);
/// await c.open(host: HostId.parse(id), profile: 'shell', cols: 80, rows: 24);
/// ```
final class TerminalController extends ChangeNotifier {
  /// Creates a controller for one tab.
  ///
  /// [selfUserId] is the person's own user ID, compared with `Host.assigned_user_id` for the
  /// warning of 24 8.4 (the server decides every right; the client only warns).
  /// [config] gives `client.terminal.scrollback_lines`. [scheduleTick] defaults to the end of
  /// the next Flutter frame; [newEphemeralKey] to `generateEphemeralKeyPair`.
  TerminalController({
    required this._api,
    required this._transport,
    required this._pins,
    required this._stepUp,
    required this._links,
    required this._clock,
    required this._prompts,
    required this._selfUserId,
    required AppConfig config,
    this._events,
    FrameTickScheduler? scheduleTick,
    EphemeralKeyFactory? newEphemeralKey,
  }) : _scheduleTick = scheduleTick ?? _endOfFrame,
       _newEphemeralKey = newEphemeralKey ?? generateEphemeralKeyPair {
    _screen = TerminalScreen(
      scrollbackLines: config.terminalScrollbackLines,
      onInput: _onRendererOutput,
    )..onResize = resize;
  }

  final TerminalServerApi _api;
  final TerminalTransport _transport;
  final HostKeyPinStore _pins;
  final TerminalStepUp _stepUp;
  final LinkOpener _links;
  final Clock _clock;
  final TerminalPrompts _prompts;
  final String _selfUserId;
  final HostEvents? _events;
  final FrameTickScheduler _scheduleTick;
  final EphemeralKeyFactory _newEphemeralKey;
  late final TerminalScreen _screen;

  TerminalTabState _state = TerminalTabState.initial;
  Host? _host;
  TerminalChannel? _channel;
  FramePump? _pump;
  StreamSubscription<HostChange>? _hostChanges;
  // The key of the handshake in progress; set right before the request and destroyed right
  // after message 2 (R-CL-15). Nothing else holds it.
  SimpleKeyPair? _ephemeral;
  int _cols = 80;
  int _rows = 24;
  int _generation = 0;
  bool _busy = false;
  BytesBuilder? _replay;

  /// Changes what the renderer sends before it reaches the host; the view sets it to the
  /// modifiers of the extra keys bar (`ExtraKeysController.transform`).
  String Function(String data)? inputTransform;

  /// The current snapshot of the tab.
  TerminalTabState get state => _state;

  /// The host as `getHost` last delivered it, null before [open].
  Host? get host => _host;

  /// The screen of the tab.
  TerminalScreen get screen => _screen;

  /// Opens a session on [host] (15 23.2 items 1 to 4).
  ///
  /// [profile] and [workdirIndex] preselect the dialog "Open terminal"; [cols] and [rows] are
  /// the size of `TerminalSessionOpen` and `start`. Never throws: every outcome is a phase of
  /// [state] (cancelled: [TerminalPhase.idle]; refused by the server: [TerminalPhase.failed]
  /// with the problem code).
  Future<void> open({
    required HostId host,
    required String profile,
    int? workdirIndex,
    required int cols,
    required int rows,
  }) async {
    if (_busy || _state.sessionId != null) return;
    _busy = true;
    try {
      _cols = cols;
      _rows = rows;
      _set(const TerminalTabState(phase: TerminalPhase.preparing));
      final checked = await _prepare(host.value);
      if (checked == null) return _set(TerminalTabState.initial);
      final h = checked.host;
      final choice = await _prompts.confirmOpen(
        h,
        OpenChoice(profile: profile, workdirIndex: workdirIndex),
      );
      if (choice == null) return _set(TerminalTabState.initial);
      _set(
        _state.copyWith(
          phase: TerminalPhase.connecting,
          profile: choice.profile,
        ),
      );
      final hash = await _freshKeyHash();
      await _stepUp(purposeText: _prompts.stepUpPurpose(h, StepUpPurpose.open));
      final grant = await _api.openSession(
        h.id,
        TerminalSessionOpen(
          (b) => b
            ..profile = choice.profile
            ..workdirIndex = choice.workdirIndex
            ..cols = cols
            ..rows = rows
            ..clientEphemeralKeyHash = hash
            ..pinChanged = checked.pinChanged ? true : null,
        ),
      );
      _set(
        _state.copyWith(
          sessionId: grant.session.id,
          openedAt: grant.session.openedAt,
          profile: grant.session.profile,
        ),
      );
      _watchHost(h.id);
      if (!await _grantMatches(grant, h)) return;
      await _connect(grant, h, checked);
    } on StepUpCancelled {
      _dropKey();
      _set(TerminalTabState.initial);
    } on Object catch (e) {
      _dropKey();
      _failed(e);
    } finally {
      _busy = false;
    }
  }

  /// Attaches to the detached session again (15 23.2 item 6): pin check against `getHost`,
  /// the warning of 24 8.4, a new ephemeral key, a step-up only when the server answers
  /// `401 step_up_required` (or before the request after `4101`), then the stream; the replay
  /// replaces the screen. Never throws.
  Future<void> attach() async {
    final sessionId = _state.sessionId;
    final host = _host;
    if (_busy || sessionId == null || host == null) return;
    if (_state.phase != TerminalPhase.detached) return;
    _busy = true;
    final before = _state;
    try {
      _set(_state.copyWith(phase: TerminalPhase.preparing, clearCauses: true));
      final checked = await _prepare(host.id);
      if (checked == null) return _set(before);
      final h = checked.host;
      _set(_state.copyWith(phase: TerminalPhase.connecting));
      final hash = await _freshKeyHash();
      final request = TerminalSessionAttach(
        (b) => b
          ..cols = _cols
          ..rows = _rows
          ..clientEphemeralKeyHash = hash
          ..pinChanged = checked.pinChanged ? true : null,
      );
      final purpose = _prompts.stepUpPurpose(h, StepUpPurpose.attach);
      if (before.detachCause == DetachCause.tokenStale) {
        await _stepUp(purposeText: purpose);
      }
      TerminalSessionGrant grant;
      try {
        grant = await _api.attachSession(sessionId, request);
      } on Object catch (e) {
        if (_problemOf(e)?.code != 'step_up_required') rethrow;
        // The server decides whether a step-up is needed (24 6.2); one retry after it.
        await _stepUp(purposeText: purpose);
        grant = await _api.attachSession(sessionId, request);
      }
      if (!await _grantMatches(grant, h)) return;
      await _connect(grant, h, checked);
    } on StepUpCancelled {
      _dropKey();
      _set(before);
    } on Object catch (e) {
      _dropKey();
      final problem = _problemOf(e);
      _set(before.copyWith(problemCode: problem?.code ?? 'attach_failed'));
    } finally {
      _busy = false;
    }
  }

  /// Detaches: closes the stream, the session keeps running on the host (15 23.2 item 6).
  Future<void> detach() async {
    if (_state.phase != TerminalPhase.connected &&
        _state.phase != TerminalPhase.exited) {
      return;
    }
    _set(
      _state.copyWith(
        phase: TerminalPhase.detached,
        detachCause: DetachCause.person,
        replaying: false,
      ),
    );
    await _closeStream();
  }

  /// Ends the session: sends `close` on a live stream, then `closeTerminalSession`.
  Future<void> end() async {
    final sessionId = _state.sessionId;
    if (sessionId == null) return;
    final pump = _pump;
    if (pump != null && _state.phase == TerminalPhase.connected) {
      await pump.sendFrame(const CloseFrame());
    }
    _set(_state.copyWith(phase: TerminalPhase.ended, replaying: false));
    await _closeStream();
    try {
      await _api.closeSession(sessionId);
    } on Object {
      // The session may already be closed (4100 crossed the request); the server is the truth
      // and reports it through terminal_session.changed.
    }
  }

  /// Sends input of the person (keys, the extra keys bar, a confirmed paste); dropped unless
  /// the tab accepts input (15 23.2 item 4).
  void input(List<int> bytes) {
    final pump = _pump;
    if (pump == null || !_state.acceptsInput || bytes.isEmpty) return;
    unawaited(pump.sendInput(bytes));
  }

  /// Records a new size of the view; at most one `resize` per frame tick reaches the host.
  void resize(int cols, int rows) {
    if (cols < 1 || rows < 1) return;
    _cols = cols;
    _rows = rows;
    final pump = _pump;
    if (pump != null && _state.phase == TerminalPhase.connected) {
      pump.requestResize(cols, rows);
    }
  }

  /// Opens a hyperlink the person confirmed; only `https` and `http` open (24 11.2).
  Future<bool> openLink(String target) async {
    if (!isOpenableLink(target)) return false;
    await _links.open(Uri.parse(target));
    return true;
  }

  @override
  void dispose() {
    unawaited(_hostChanges?.cancel());
    unawaited(_channel?.close());
    _dropKey();
    _screen.dispose();
    super.dispose();
  }

  // ---- steps ---------------------------------------------------------------------------

  Future<_Checked?> _prepare(String hostId) async {
    final h = await _api.getHost(hostId);
    _host = h;
    notifyListeners();
    final key = h.noisePublicKey;
    final fingerprint = h.keyFingerprint;
    if (key == null || fingerprint == null) {
      throw const ApiProblem(status: 409, code: 'host_unavailable');
    }
    final pin = await _pins.read(h.id);
    final check = checkPin(pin, key, fingerprint);
    var pinChanged = false;
    if (check is PinChanged) {
      if (!await _prompts.confirmKeyChange(h, check)) return null;
      pinChanged = true;
    }
    final assignee = h.assignedUserId;
    if (assignee != null && assignee != _selfUserId) {
      // 24 8.4: the warning comes before the step-up; no step-up without the confirmation.
      if (!await _prompts.confirmForeignHost(h)) return null;
    }
    return _Checked(h, writePin: check is! PinMatches, pinChanged: pinChanged);
  }

  Future<String> _freshKeyHash() async {
    _dropKey();
    final pair = await _newEphemeralKey();
    _ephemeral = pair;
    final public = await pair.extractPublicKey();
    return ephemeralKeyHash(public.bytes);
  }

  Future<bool> _grantMatches(TerminalSessionGrant grant, Host checked) async {
    if (sameNoiseKey(grant.noisePublicKey, checked.noisePublicKey!) &&
        grant.keyGeneration == checked.keyGeneration) {
      return true;
    }
    // 15 23.2 item 3: a different key than the checked one aborts and closes the session.
    _dropKey();
    _set(
      _state.copyWith(
        phase: TerminalPhase.blocked,
        blockCause: BlockCause.keyMismatch,
      ),
    );
    try {
      await _api.closeSession(grant.session.id);
    } on Object {
      // The block stays either way; the server ends an unused opening after
      // terminal.open_timeout.
    }
    return false;
  }

  Future<void> _connect(
    TerminalSessionGrant grant,
    Host host,
    _Checked checked,
  ) async {
    final generation = ++_generation;
    final TerminalChannel channel;
    try {
      channel = await _transport.connect(grant.streamPath);
    } on Object {
      _dropKey();
      return _set(
        _state.copyWith(
          phase: TerminalPhase.detached,
          detachCause: DetachCause.connectionLost,
        ),
      );
    }
    if (generation != _generation || _state.phase != TerminalPhase.connecting) {
      _dropKey();
      await channel.close();
      return;
    }
    _channel = channel;
    unawaited(_run(channel, generation, grant, host, checked));
  }

  Future<void> _run(
    TerminalChannel channel,
    int generation,
    TerminalSessionGrant grant,
    Host host,
    _Checked checked,
  ) async {
    NkInitiator? initiator;
    var stage = _Stage.hello;
    BlockCause? failure;
    try {
      await for (final message in channel.messages) {
        if (generation != _generation) break;
        switch (stage) {
          case _Stage.hello:
            if (message is! String) {
              failure = BlockCause.protocolError;
              break;
            }
            final hello = _helloOf(message);
            if (hello == null) {
              // terminal.error or an unknown text: the server closes the stream next.
              continue;
            }
            final versions = hello['protocol_versions'];
            if (versions is! List ||
                !versions.contains(terminalStreamProtocolVersion)) {
              failure = BlockCause.protocolError;
              break;
            }
            final identify = await _transport.identify(hello);
            channel.sendText(jsonEncode({'op': 'identify', 'd': identify}));
            final key = _ephemeral;
            if (key == null) {
              failure = BlockCause.handshakeFailed;
              break;
            }
            initiator = await NkInitiator.start(
              hostStaticPublicKey: decodeNoiseKey(grant.noisePublicKey)!,
              sessionId: uuidToBytes(grant.session.id),
              hostId: uuidToBytes(host.id),
              ephemeral: key,
            );
            channel.sendBinary(
              await initiator.writeMessage1(utf8.encode(grant.sessionToken)),
            );
            stage = _Stage.message2;
          case _Stage.message2:
            if (message is! Uint8List) {
              failure = BlockCause.protocolError;
              break;
            }
            final result = await initiator!.readMessage2(message);
            initiator = null;
            // R-CL-15: the private part is not needed after message 2.
            _dropKey();
            final pump = FramePump(
              transport: result.transport,
              send: channel.sendBinary,
              scheduleTick: _scheduleTick,
              onError: (_) =>
                  _blockAndClose(channel, BlockCause.handshakeFailed),
            );
            _pump = pump;
            await pump.sendFrame(StartFrame(_cols, _rows));
            stage = _Stage.frames;
            _set(
              _state.copyWith(
                phase: TerminalPhase.connected,
                clearCauses: true,
                replaying: false,
              ),
            );
            if (checked.writePin) {
              await _pins.write(
                HostKeyPin(
                  hostId: host.id,
                  noisePublicKey: grant.noisePublicKey,
                  keyGeneration: grant.keyGeneration,
                  fingerprint: grant.keyFingerprint,
                  firstSeen: _clock.now(),
                ),
              );
            }
          case _Stage.frames:
            if (message is! Uint8List) {
              failure = BlockCause.protocolError;
              break;
            }
            _onFrame(await _pump!.receive(message));
        }
        if (failure != null) break;
      }
    } on Object {
      // A Noise or frame error is tampering (24 5.3, 5.4): hard warning, no retry. The error
      // object is dropped on purpose; it may hold bytes of the stream.
      failure = BlockCause.handshakeFailed;
    }
    _dropKey();
    if (generation != _generation) return;
    _pump = null;
    _channel = null;
    if (failure != null) {
      await channel.close();
      return _set(
        _state.copyWith(
          phase: TerminalPhase.blocked,
          blockCause: failure,
          replaying: false,
        ),
      );
    }
    _afterClose(channel.closeCode, channel.closeReason);
  }

  void _onFrame(TerminalFrame frame) {
    switch (frame) {
      case DataFrame(:final bytes):
        final replay = _replay;
        if (replay != null) {
          replay.add(bytes);
        } else {
          _screen.write(bytes);
        }
      case ExitFrame(:final code):
        _set(_state.copyWith(phase: TerminalPhase.exited, exitCode: code));
      case ReplayBeginFrame():
        _replay = BytesBuilder(copy: false);
        _set(_state.copyWith(replaying: true));
      case ReplayEndFrame():
        final replay = _replay;
        _replay = null;
        if (replay != null) _screen.replaceWith(replay.takeBytes());
        _set(_state.copyWith(replaying: false));
      case StartFrame() || ResizeFrame() || CloseFrame():
        // decodeFrame refuses device frames from the host; unreachable.
        break;
    }
  }

  void _afterClose(int? code, String? reason) {
    final phase = _state.phase;
    if (phase == TerminalPhase.ended ||
        phase == TerminalPhase.blocked ||
        (phase == TerminalPhase.detached &&
            _state.detachCause == DetachCause.person)) {
      return;
    }
    final reaction = closeReactionFor(code, reason);
    _replay = null;
    _set(
      _state.copyWith(
        phase: reaction.phase,
        detachCause: reaction.detachCause,
        blockCause: reaction.blockCause,
        closeReason: reaction.closeReason,
        replaying: false,
      ),
    );
  }

  void _blockAndClose(TerminalChannel channel, BlockCause cause) {
    _generation++;
    _pump = null;
    _channel = null;
    _set(_state.copyWith(phase: TerminalPhase.blocked, blockCause: cause));
    unawaited(channel.close());
  }

  Future<void> _closeStream() async {
    _generation++;
    _pump = null;
    _replay = null;
    _dropKey();
    final channel = _channel;
    _channel = null;
    if (channel != null) await channel.close();
  }

  void _watchHost(String hostId) {
    final events = _events;
    if (events == null || _hostChanges != null) return;
    _hostChanges = events.watch(hostId).listen((change) {
      if (change is TerminalSessionUpdated &&
          change.closed &&
          change.sessionId == _state.sessionId &&
          _state.phase != TerminalPhase.blocked &&
          _state.phase != TerminalPhase.ended) {
        final reaction = closeReactionFor(
          StreamCloseCode.terminalClosed,
          change.closeReason,
        );
        _set(
          _state.copyWith(
            phase: reaction.phase,
            blockCause: reaction.blockCause,
            closeReason: reaction.closeReason,
          ),
        );
        unawaited(_closeStream());
      }
    });
  }

  void _onRendererOutput(String data) {
    final transform = inputTransform;
    input(utf8.encode(transform == null ? data : transform(data)));
  }

  void _failed(Object error) {
    final problem = _problemOf(error);
    _set(
      _state.copyWith(
        phase: _state.sessionId == null
            ? TerminalPhase.failed
            : TerminalPhase.detached,
        detachCause: _state.sessionId == null
            ? null
            : DetachCause.connectionLost,
        problemCode: problem?.code ?? 'open_failed',
      ),
    );
  }

  void _dropKey() {
    final key = _ephemeral;
    _ephemeral = null;
    if (key != null && !key.hasBeenDestroyed) key.destroy();
  }

  void _set(TerminalTabState next) {
    _state = next;
    notifyListeners();
  }

  static ApiProblem? _problemOf(Object error) =>
      error is ApiProblem ? error : ApiProblem.of(error);

  static Map<String, Object?>? _helloOf(String text) {
    try {
      final json = jsonDecode(text);
      if (json is Map && json['op'] == 'hello' && json['d'] is Map) {
        return (json['d'] as Map).cast<String, Object?>();
      }
    } on FormatException {
      return null;
    }
    return null;
  }

  static void _endOfFrame(void Function() callback) {
    SchedulerBinding.instance.scheduleFrameCallback((_) => callback());
    SchedulerBinding.instance.ensureVisualUpdate();
  }
}

enum _Stage { hello, message2, frames }

final class _Checked {
  const _Checked(this.host, {required this.writePin, required this.pinChanged});

  final Host host;
  final bool writePin;
  final bool pinChanged;
}
