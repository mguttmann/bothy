// SPDX-License-Identifier: Apache-2.0

/// The gateway connection of the apps: transport, session, heartbeat, backoff and the close
/// codes, as the state machine of 15 7.2 (brief M4-13).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' hide Heartbeat;
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'backoff.dart';
import 'close_codes.dart';
import 'connection_state.dart';
import 'focus_manager.dart';
import 'frames.dart';
import 'heartbeat.dart';
import 'ports.dart';
import 'session.dart';
import 'session_listener.dart';
import 'typing_throttle.dart';

/// The one gateway connection of a device (04 6.1): it never opens a second socket while one
/// is open or opening.
///
/// After [start] it connects when the network is available and, on mobile, the app is in the
/// foreground; it answers `hello` through [GatewaySession], sends heartbeats ([Heartbeat]),
/// reacts to every close by [reactTo] and waits by [ReconnectBackoff]. A network change and
/// the return to the foreground cut a backoff wait short without raising the attempt counter
/// (04 6.5). With `keepConnectedInBackground` (desktop) the connection never enters
/// [ConnectionPhase.suspended]; on mobile it closes with `1000` in the background and
/// reconnects in the foreground (15 7.2).
///
/// Catch-up, the dispatcher and the outbox plug in through [SessionListener] (M4-14, M4-15);
/// without a listener `ready` and `resumed` lead straight to [ConnectionPhase.live].
///
/// Example:
/// ```dart
/// final connection = GatewayConnection(
///   transport: IoGatewayTransport(),
///   gatewayUrl: Uri.parse('wss://chat.example.com/api/v1/gateway'),
///   auth: authSession,
///   syncState: db.syncStateDao,
///   config: appConfig,
///   clock: const SystemClock(),
///   random: SystemRandomSource(),
///   network: network,
///   lifecycle: lifecycle,
///   client: GatewayClientInfo((b) => b
///     ..platform = DevicePlatform.macos
///     ..version = '1.0.0'),
///   keepConnectedInBackground: true,
///   listener: catchUp,
/// );
/// connection.states.listen(showBanner);
/// connection.start();
/// ```
final class GatewayConnection {
  /// Creates the connection; nothing happens before [start].
  ///
  /// [config] supplies `sync.reconnect_backoff_base`, `sync.reconnect_backoff_cap`,
  /// `sync.typing_min_interval` and `sync.max_focused_threads` (D-W-115) and the attempt
  /// deadline `client.sync.connect_timeout` (15 7.2); every other interval comes from the
  /// server. [logHook] receives one line per notable step, never a token,
  /// proof or frame content.
  GatewayConnection({
    required this._transport,
    required this._gatewayUrl,
    required this._auth,
    required SyncStateDao syncState,
    required AppConfig config,
    required this._clock,
    required this._random,
    required this._network,
    required this._lifecycle,
    required GatewayClientInfo client,
    this._keepConnectedInBackground = false,
    this._listener,
    this._logHook,
  }) : _connectTimeout = config.syncConnectTimeout,
       _backoff = ReconnectBackoff(
         base: config.reconnectBackoffBase,
         cap: config.reconnectBackoffCap,
         random: _random,
       ),
       _session = GatewaySession(
         auth: _auth,
         syncState: syncState,
         gatewayUrl: _gatewayUrl,
         client: client,
         clock: _clock,
       ) {
    focus = FocusManager(
      limit: config.maxFocusedThreads,
      trySend: _sendIfIdentified,
    );
    typing = TypingThrottle(
      clock: _clock,
      interval: config.typingMinInterval,
      trySend: _sendIfIdentified,
    );
    _control = _Control(this);
  }

  final GatewayTransport _transport;
  final Uri _gatewayUrl;
  final AuthSession _auth;
  final Clock _clock;
  final RandomSource _random;
  final NetworkStatus _network;
  final AppLifecycle _lifecycle;
  final bool _keepConnectedInBackground;
  final SessionListener? _listener;
  final void Function(String line)? _logHook;
  final ReconnectBackoff _backoff;
  final GatewaySession _session;

  /// `client.sync.connect_timeout`: the deadline of one attempt until `ready` or `resumed`.
  final Duration _connectTimeout;
  late final _Control _control;

  /// The focus of this connection.
  late final FocusManager focus;

  /// The typing throttle of this connection.
  late final TypingThrottle typing;

  final StreamController<ConnectionState> _states =
      StreamController<ConnectionState>.broadcast(sync: true);
  ConnectionState _state = const ConnectionState(ConnectionPhase.offline);

  StreamSubscription<bool>? _networkSub;
  StreamSubscription<AppActivity>? _lifecycleSub;
  bool _started = false;
  bool _stopped = false;

  GatewayChannel? _channel;
  StreamSubscription<String>? _frameSub;

  /// Counts sockets; a callback of an older socket compares and does nothing.
  int _channelGen = 0;

  /// Counts waits; a wait that completes after a newer decision does nothing.
  int _waitGen = 0;

  /// True from the start of an attempt until its socket is closed and handled.
  bool _attempting = false;

  /// True between `ready` or `resumed` and the end of the socket.
  bool _identified = false;

  /// The reaction for a close the client started itself.
  CloseReaction? _ownReaction;

  Future<void> _work = Future<void>.value();
  Heartbeat? _heartbeat;
  Duration _heartbeatInterval = Duration.zero;
  GlobalSeq? _lastProcessedSeq;
  int _authFailures = 0;
  int _decodeErrors = 0;

  /// The current state.
  ConnectionState get state => _state;

  /// Every state change (broadcast, delivered synchronously).
  Stream<ConnectionState> get states => _states.stream;

  /// The number `n` of the next backoff attempt (04 6.5).
  int get backoffAttempt => _backoff.attempt;

  /// Frames of the server that could not be decoded and were skipped (Design note 1).
  int get decodeErrors => _decodeErrors;

  /// The `sync_epoch` of the current `hello`, `ready` or `resumed`.
  String? get syncEpoch => _session.syncEpoch;

  bool get _foreground => _lifecycle.current == AppActivity.foreground;

  /// Starts: subscribes to network and lifecycle and connects unless the device is offline or
  /// a mobile app is in the background. A second call does nothing.
  void start() {
    if (_started) return;
    _started = true;
    _networkSub = _network.changes.listen(_onNetwork);
    _lifecycleSub = _lifecycle.changes.listen(_onLifecycle);
    if (!_keepConnectedInBackground && !_foreground) {
      _set(ConnectionPhase.suspended);
      return;
    }
    _connect();
  }

  /// Connects now because the person asked (after `4017`, `4018`, `4015` or [stop]); does
  /// nothing while a socket is open or opening, after a sign-out or when an update is required.
  Future<void> connectNow() async {
    if (!_started) return start();
    _stopped = false;
    if (_channel != null || _attempting) return;
    if (_state.phase == ConnectionPhase.signedOut ||
        _state.phase == ConnectionPhase.updateRequired) {
      return;
    }
    _connect();
  }

  /// Closes the socket with `1000` and stays offline until [connectNow].
  Future<void> stop() async {
    _stopped = true;
    _waitGen++;
    _heartbeat?.stop();
    final ch = _channel;
    if (ch != null) {
      _ownReaction = const CloseReaction(
        code: 1000,
        phase: ConnectionPhase.offline,
        reason: ConnectionReason.none,
        reconnect: Reconnect.never,
      );
      await ch.close(1000);
    } else if (_attempting) {
      _channelGen++;
      _attempting = false;
    }
    _set(ConnectionPhase.offline);
  }

  /// Stops and releases the subscriptions; the connection is unusable afterwards.
  Future<void> dispose() async {
    await stop();
    await _work;
    await _networkSub?.cancel();
    await _lifecycleSub?.cancel();
    await _frameSub?.cancel();
    await _states.close();
  }

  // -------------------------------------------------------------------------------------------
  // Attempts

  void _connect() {
    if (_stopped || _attempting) return;
    _waitGen++;
    if (!_keepConnectedInBackground && !_foreground) {
      // Mobile has no WebSocket in the background (15 7.2 `Suspended`); every path that would
      // reconnect there (a backoff timer, a token renewal, an approved device) waits for the
      // foreground instead.
      _set(ConnectionPhase.suspended);
      return;
    }
    if (!_network.online) {
      _set(ConnectionPhase.offline, ConnectionReason.networkUnavailable);
      return;
    }
    _attempting = true;
    _identified = false;
    _ownReaction = null;
    final gen = ++_channelGen;
    _set(ConnectionPhase.connecting);
    unawaited(_open(gen));
    unawaited(_clock.delay(_connectTimeout).then((_) => _onDeadline(gen)));
  }

  /// Ends the attempt [gen] when it has not reached `ready` or `resumed` within
  /// `client.sync.connect_timeout` (15 7.2): a stalled connect, a socket without `hello` or an
  /// unanswered `identify` or `resume`. The generation is invalidated, so a late socket or frame
  /// of the attempt does nothing, and the attempt takes the backoff of a failed connect once its
  /// socket is closed: the attempt stays open until then, so no second socket opens (04 6.1).
  void _onDeadline(int gen) {
    if (gen != _channelGen || !_attempting || _identified || _stopped) return;
    // A close the client started itself (sign-out, update required, background) runs already.
    if (_ownReaction != null) return;
    final ch = _channel;
    // The server closed first: its code waits in the queue and decides the reaction.
    if (ch?.closeCode != null) return;
    _log('connect attempt timed out');
    final sub = _frameSub;
    final mine = ++_channelGen;
    _channel = null;
    _frameSub = null;
    unawaited(_endTimedOut(mine, ch, sub));
  }

  /// Closes the socket of a timed-out attempt, bounded by `client.sync.connect_timeout` again,
  /// then ends the attempt with the backoff of a failed connect unless something newer took
  /// over (a stop or the background).
  Future<void> _endTimedOut(
    int mine,
    GatewayChannel? ch,
    StreamSubscription<String>? sub,
  ) async {
    if (ch != null) {
      unawaited(sub?.cancel());
      await Future.any<void>([
        ch.close(1000).catchError((Object _) {}),
        _clock.delay(_connectTimeout),
      ]);
    }
    if (mine != _channelGen || _stopped) return;
    _attempting = false;
    _enqueue(() => _apply(reactTo(null, null, foreground: _foreground)));
  }

  Future<void> _open(int gen) async {
    final GatewayChannel ch;
    try {
      ch = await _transport.connect(_gatewayUrl);
    } on Object catch (e) {
      if (gen != _channelGen) return;
      _log('connect failed: ${e.runtimeType}');
      _attempting = false;
      _enqueue(() => _apply(reactTo(null, null, foreground: _foreground)));
      return;
    }
    if (gen != _channelGen) {
      await ch.close(1000);
      return;
    }
    _channel = ch;
    _frameSub = ch.frames.listen(
      (text) => _onText(gen, text),
      onError: (Object e) => _log('socket error: ${e.runtimeType}'),
      onDone: () => _enqueue(() => _onDone(gen)),
    );
  }

  /// Runs [step] after every earlier frame was handled, so frames apply in order.
  void _enqueue(Future<void> Function() step) {
    _work = _work
        .then((_) => step())
        .then<void>(
          (_) {},
          onError: (Object e) =>
              _log('frame handling failed: ${e.runtimeType}'),
        );
  }

  // -------------------------------------------------------------------------------------------
  // Frames

  /// Decodes [text] at once; the control frames `heartbeat_ack` and `reauth_required` act
  /// directly, every other frame waits in the ordered queue behind the listener steps, so a
  /// long snapshot or catch-up step of M4-14 cannot delay them past their deadlines (04 6.3
  /// items 5 and 6; red team of M4-13).
  void _onText(int gen, String text) {
    if (gen != _channelGen) return;
    final GatewayFrame? frame;
    try {
      frame = decodeFrame(text);
    } on FrameDecodeError catch (e) {
      // Closing with a protocol code is the server's role (04 6.4); the client skips the frame.
      _decodeErrors++;
      _log('frame skipped: ${e.cause}');
      return;
    }
    switch (frame) {
      case null:
        return;
      case HeartbeatAckFrame():
        _heartbeat?.acknowledge();
      case ReauthRequiredFrame(:final model):
        unawaited(
          _onReauth(
            gen,
            model,
          ).catchError((Object e) => _log('reauth failed: ${e.runtimeType}')),
        );
      case _:
        _enqueue(() => _onFrame(gen, frame!));
    }
  }

  Future<void> _onFrame(int gen, GatewayFrame frame) async {
    if (gen != _channelGen) return;
    switch (frame) {
      case HelloFrame(:final model):
        await _onHello(gen, model);
      case ReadyFrame(:final model):
        await _onReady(gen, model);
      case ResumedFrame(:final model):
        await _onResumed(gen, model);
      case HeartbeatAckFrame() || ReauthRequiredFrame():
        // Handled in _onText, outside the queue.
        break;
      case ErrorFrame(:final model):
        if (model.op == 'focus' && model.code == 'validation_failed') {
          focus.onRejected();
        }
        _log('error frame for ${model.op}: ${model.code}');
        await _toListener(frame);
      case DispatchFrame() ||
          CursorFrame() ||
          CatchupCompleteFrame() ||
          ResyncRequiredFrame():
        await _toListener(frame);
      case IdentifyFrame() ||
          ResumeFrame() ||
          HeartbeatFrame() ||
          ReauthFrame() ||
          FocusFrame() ||
          TypingStartFrame() ||
          TypingStopFrame():
        // Client ops never come from the server; ignored like an unknown op.
        break;
    }
  }

  Future<void> _toListener(GatewayFrame frame) async {
    final listener = _listener;
    if (listener != null && _identified) {
      await listener.onFrame(frame, _control);
    }
  }

  Future<void> _onHello(int gen, GatewayHello hello) async {
    if (!_session.supports(hello)) {
      _log('protocol version $gatewayProtocolVersion not offered');
      await _closeOwn(
        const CloseReaction(
          code: 1000,
          phase: ConnectionPhase.updateRequired,
          reason: ConnectionReason.protocolUnsupported,
          reconnect: Reconnect.never,
        ),
      );
      return;
    }
    _heartbeatInterval = Duration(milliseconds: hello.heartbeatIntervalMs);
    final GatewayFrame answer;
    try {
      answer = await _session.answerHello(hello);
    } on AuthException catch (e) {
      if (gen != _channelGen) return;
      await _closeOwn(_authFailure(e));
      return;
    }
    if (gen != _channelGen) return;
    _send(answer);
    _log('sent ${answer.op}');
    _set(ConnectionPhase.identifying);
  }

  Future<void> _onReady(int gen, GatewayReady ready) async {
    await _session.onReady(ready);
    if (gen != _channelGen) return;
    await _sessionStarted();
    focus.onReady();
    final listener = _listener;
    if (listener == null) {
      _set(ConnectionPhase.live);
      return;
    }
    _set(ConnectionPhase.catchingUp);
    await listener.onReady(ready, _control);
  }

  Future<void> _onResumed(int gen, GatewayResumed resumed) async {
    _session.onResumed(resumed);
    if (gen != _channelGen) return;
    await _sessionStarted();
    focus.onResumed();
    final listener = _listener;
    if (listener == null) {
      _set(ConnectionPhase.live);
      return;
    }
    _set(ConnectionPhase.catchingUp);
    await listener.onResumed(resumed, _control);
  }

  /// Common part of `ready` and `resumed`: backoff and renewal count reset (04 6.5), heartbeat.
  Future<void> _sessionStarted() async {
    _identified = true;
    _backoff.reset();
    _authFailures = 0;
    _lastProcessedSeq = await _session.storedCursor();
    final gen = _channelGen;
    _heartbeat?.stop();
    _heartbeat = Heartbeat(
      clock: _clock,
      random: _random,
      interval: _heartbeatInterval,
      send: () {
        if (gen != _channelGen) return;
        final seq = _lastProcessedSeq?.value ?? 0;
        _send(HeartbeatFrame(GatewayHeartbeat((b) => b..lastSeq = seq)));
      },
      onMissedAck: () {
        if (gen != _channelGen) return;
        _log('heartbeat_ack missing');
        unawaited(
          _closeOwn(
            const CloseReaction(
              code: null,
              phase: ConnectionPhase.backoff,
              reason: ConnectionReason.none,
              reconnect: Reconnect.backoff,
            ),
          ),
        );
      },
    )..start();
  }

  Future<void> _onReauth(int gen, GatewayReauthRequired required) async {
    final ReauthFrame frame;
    try {
      frame = await _session.answerReauth(required);
    } on SessionEnded catch (e) {
      if (gen == _channelGen) await _closeOwn(_authFailure(e));
      return;
    } on NotSignedIn catch (e) {
      if (gen == _channelGen) await _closeOwn(_authFailure(e));
      return;
    } on AuthException catch (e) {
      // Transient: the server closes with 4011 at expiry, which renews again (04 6.4).
      _log('reauth refresh failed: $e');
      return;
    }
    if (gen != _channelGen) return;
    _send(frame);
    _log('sent reauth');
  }

  /// The reaction to a failed token in `hello`: sign-out when the session ended, else backoff.
  CloseReaction _authFailure(AuthException e) => switch (e) {
    SessionEnded() || NotSignedIn() => const CloseReaction(
      code: 1000,
      phase: ConnectionPhase.signedOut,
      reason: ConnectionReason.none,
      reconnect: Reconnect.never,
    ),
    _ => const CloseReaction(
      code: 1000,
      phase: ConnectionPhase.backoff,
      reason: ConnectionReason.none,
      reconnect: Reconnect.backoff,
    ),
  };

  // -------------------------------------------------------------------------------------------
  // Closing

  /// Closes the socket with `1000` from the client side and applies [reaction] when it is done.
  Future<void> _closeOwn(CloseReaction reaction) async {
    final ch = _channel;
    // A socket with a close code was already closed by the server; its code wins (for example
    // 4012 arriving just before the app goes to the background).
    if (ch == null || ch.closeCode != null) return;
    _ownReaction = reaction;
    _heartbeat?.stop();
    if (reaction.phase != ConnectionPhase.backoff) {
      _set(reaction.phase, reaction.reason);
    }
    await ch.close(1000);
  }

  Future<void> _onDone(int gen) async {
    if (gen != _channelGen) return;
    final ch = _channel;
    _channel = null;
    _frameSub = null;
    _attempting = false;
    _heartbeat?.stop();
    typing.reset();
    final wasIdentified = _identified;
    _identified = false;
    if (wasIdentified) {
      // A failing store or listener must not keep the connection from reacting to the close.
      try {
        await _session.markDisconnected();
        _listener?.onDisconnected();
      } on Object catch (e) {
        _log('disconnect bookkeeping failed: ${e.runtimeType}');
      }
    }
    final reaction =
        _ownReaction ??
        reactTo(ch?.closeCode, ch?.closeReason, foreground: _foreground);
    _ownReaction = null;
    _log('closed: ${ch?.closeCode ?? 'no code'}');
    if (reaction.unexpected) _log('unexpected close code ${reaction.code}');
    await _apply(reaction);
  }

  Future<void> _apply(CloseReaction reaction) async {
    if (_stopped) return;
    if (reaction.identify) _session.forceIdentify = true;
    if (reaction.signOut) {
      _set(reaction.phase, reaction.reason);
      // Deletes the tokens and the device ID and emits SignedOut(sessionRevoked), whose
      // deleteLocalData asks the app to delete the local data (R-SY-14).
      await _auth.sessionRevoked();
      return;
    }
    if (reaction.renewToken) {
      await _renewThenConnect(reaction);
      return;
    }
    switch (reaction.reconnect) {
      case Reconnect.backoff:
        _wait(reaction.retryAfter ?? _backoff.next(), reaction.reason);
      case Reconnect.serverRestart:
        // 4013: uniform in [0, base], the counter stays (04 6.4).
        _wait(_backoff.base * _random.nextDouble(), reaction.reason);
      case Reconnect.immediate:
        _set(ConnectionPhase.backoff, reaction.reason);
        _connect();
      case Reconnect.afterPendingDevice:
        _set(reaction.phase, reaction.reason);
        unawaited(_pollPendingDevice());
      case Reconnect.onForeground
          when reaction.phase == ConnectionPhase.suspended && _foreground:
        // The app came back while its own background close was still running.
        _connect();
      case Reconnect.onForeground || Reconnect.onUserAction || Reconnect.never:
        _set(reaction.phase, reaction.reason);
    }
  }

  /// `4004` and `4011`: renew the token once, then reconnect at once; a second renewal before
  /// the next `ready` or `resumed` waits the backoff, so a server that keeps rejecting is not
  /// flooded.
  Future<void> _renewThenConnect(CloseReaction reaction) async {
    _set(ConnectionPhase.backoff, reaction.reason);
    _authFailures++;
    try {
      await _auth.refresh.refreshNow();
    } on SessionEnded {
      _set(ConnectionPhase.signedOut, reaction.reason);
      return;
    } on NotSignedIn {
      _set(ConnectionPhase.signedOut, reaction.reason);
      return;
    } on AuthException {
      _wait(_backoff.next(), reaction.reason);
      return;
    }
    if (_stopped) return;
    if (_authFailures > 1) {
      _wait(_backoff.next(), reaction.reason);
      return;
    }
    _connect();
  }

  void _wait(Duration wait, ConnectionReason reason) {
    if (!_network.online) {
      _set(ConnectionPhase.offline, ConnectionReason.networkUnavailable);
      return;
    }
    _set(ConnectionPhase.backoff, reason);
    final w = ++_waitGen;
    unawaited(
      _clock.delay(wait).then((_) {
        if (w == _waitGen && _state.phase == ConnectionPhase.backoff) {
          _connect();
        }
      }),
    );
  }

  /// `4016`: polls the device through `LoginService.pollPendingDevice` and reconnects once it
  /// is `active`; a rejected device ends signed out (08 5.2).
  Future<void> _pollPendingDevice() async {
    final device = _auth.session?.deviceId;
    if (device == null) return;
    final w = ++_waitGen;
    final outcome = await _auth.login.pollPendingDevice(
      device,
      isCancelled: () => w != _waitGen || _stopped,
    );
    if (w != _waitGen || _stopped) return;
    switch (outcome) {
      case LoginSignedIn():
        _connect();
      case LoginFailed(reason: 'cancelled'):
        break;
      case _:
        _set(ConnectionPhase.signedOut, ConnectionReason.revoked);
    }
  }

  // -------------------------------------------------------------------------------------------
  // Network and lifecycle

  void _onNetwork(bool online) {
    if (_stopped) return;
    if (_channel != null || _attempting) return;
    final phase = _state.phase;
    if (!online) {
      if (phase == ConnectionPhase.backoff) {
        _waitGen++;
        _set(ConnectionPhase.offline, ConnectionReason.networkUnavailable);
      }
      return;
    }
    // 04 6.5: a network change triggers an immediate attempt without raising the counter.
    if (phase == ConnectionPhase.backoff ||
        (phase == ConnectionPhase.offline &&
            _state.reason == ConnectionReason.networkUnavailable)) {
      _connect();
    }
  }

  void _onLifecycle(AppActivity activity) {
    if (_stopped) return;
    if (activity == AppActivity.background) {
      if (_keepConnectedInBackground) return;
      _goSuspended();
      return;
    }
    if (_channel != null || _attempting) return;
    final s = _state;
    // 04 6.5 and 15 7.2: the foreground connects at once without raising the counter; 4017
    // and a server 1000 in the background wait for exactly this.
    if (s.phase == ConnectionPhase.suspended ||
        s.phase == ConnectionPhase.backoff ||
        (s.phase == ConnectionPhase.offline &&
            (s.reason == ConnectionReason.deviceReplaced ||
                s.reason == const ConnectionReason.closedByServer(1000) ||
                s.reason == ConnectionReason.networkUnavailable))) {
      _connect();
    }
  }

  /// Mobile in the background: no WebSocket (15 7.2 `Suspended`), close with `1000`.
  void _goSuspended() {
    const suspended = CloseReaction(
      code: 1000,
      phase: ConnectionPhase.suspended,
      reason: ConnectionReason.none,
      reconnect: Reconnect.onForeground,
    );
    final phase = _state.phase;
    if (_channel != null) {
      unawaited(_closeOwn(suspended));
    } else if (_attempting) {
      _channelGen++;
      _attempting = false;
      _set(ConnectionPhase.suspended);
    } else if (phase == ConnectionPhase.backoff ||
        (phase == ConnectionPhase.offline &&
            (_state.reason == ConnectionReason.networkUnavailable ||
                _state.reason ==
                    const ConnectionReason.closedByServer(1000)))) {
      _waitGen++;
      _set(ConnectionPhase.suspended);
    }
  }

  // -------------------------------------------------------------------------------------------
  // Sending and state

  void _send(GatewayFrame frame) {
    final ch = _channel;
    if (ch == null) return;
    try {
      ch.send(encodeFrame(frame));
    } on Object catch (e) {
      _log('send failed: ${e.runtimeType}');
    }
  }

  bool _sendIfIdentified(GatewayFrame frame) {
    if (!_identified || _channel == null) return false;
    _send(frame);
    return true;
  }

  void _set(
    ConnectionPhase phase, [
    ConnectionReason reason = ConnectionReason.none,
  ]) {
    final next = ConnectionState(phase, reason);
    if (next == _state) return;
    _state = next;
    _log('state $next');
    if (!_states.isClosed) _states.add(next);
  }

  void _log(String line) => _logHook?.call('gateway: $line');
}

final class _Control implements SessionControl {
  _Control(this._c);

  final GatewayConnection _c;

  static const Set<ConnectionPhase> _allowed = {
    ConnectionPhase.catchingUp,
    ConnectionPhase.resyncing,
    ConnectionPhase.live,
  };

  @override
  void setPhase(ConnectionPhase phase) {
    if (!_allowed.contains(phase)) {
      throw ArgumentError.value(phase, 'phase', 'not a phase of the listener');
    }
    if (_c._identified) _c._set(phase);
  }

  @override
  bool send(GatewayFrame frame) => _c._sendIfIdentified(frame);

  @override
  set lastProcessedSeq(GlobalSeq seq) => _c._lastProcessedSeq = seq;

  @override
  String? get syncEpoch => _c._session.syncEpoch;
}
