// SPDX-License-Identifier: Apache-2.0

/// The ports of the terminal view (docs/spec/15-client-architecture.md 23.1): every app
/// implements them its own way (desktop and mobile app: own WebSocket with DPoP `identify`,
/// pins in `local_store`, host events from `sync_engine`; web console: browser WebSocket,
/// IndexedDB pins, reload through REST). The controller sees nothing else of the app.
library;

import 'dart:typed_data';

import 'package:api_client/api_client.dart'
    show
        Host,
        TerminalSession,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionOpen;

/// The REST operations the terminal view calls (docs/api/openapi.yaml `getHost`,
/// `openTerminalSession`, `attachTerminalSession`, `closeTerminalSession`).
///
/// Named `TerminalServerApi` because `package:api_client` already exports the generated class
/// `TerminalApi`. An implementation throws the error of the request unchanged: a `DioException`
/// carrying an `ApiProblem` (the client of `createApiClient`) or an `ApiProblem` itself; the
/// controller reads `401 step_up_required`, `403 terminal_denied` and `409 host_unavailable`
/// from it.
///
/// An app must not let an interceptor answer `401 step_up_required` of
/// [attachSession] on its own: the controller runs the step-up itself, after the warning of
/// docs/spec/24-terminal.md 8.4.
abstract interface class TerminalServerApi {
  /// Loads the host [hostId] (`getHost`): key, fingerprint, key generation, assignee, login
  /// badge and connection.
  Future<Host> getHost(String hostId);

  /// Opens a session on [hostId] (`openTerminalSession`) after a fresh step-up.
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  );

  /// Attaches to the session [sessionId] again (`attachTerminalSession`) with a new
  /// ephemeral key; the server answers `401 step_up_required` when a step-up is needed
  /// (`terminal.quick_reattach_window`, docs/spec/24-terminal.md 6.2).
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  );

  /// Ends the session [sessionId] (`closeTerminalSession`).
  Future<TerminalSession> closeSession(String sessionId);
}

/// The step-up of the person (docs/spec/08-identity.md 6.3): in the apps the tear-off
/// `session.stepUp.confirm` of `package:auth`, in the console the passkey step-up (M6).
///
/// Completes when the server confirmed the step-up; throws `StepUpCancelled` of `package:auth`
/// when the person declined, any other error when the confirmation failed.
typedef TerminalStepUp = Future<void> Function({required String purposeText});

/// Opens the WebSocket of one terminal stream (asyncapi channel `terminalStream`).
abstract interface class TerminalTransport {
  /// Connects to [streamPath] (the `stream_path` of the grant, below the server origin).
  ///
  /// Throws when the connection cannot be opened; the controller then shows the session as
  /// detached.
  Future<TerminalChannel> connect(String streamPath);

  /// Returns the member `d` of `terminal.identify` for [hello] (the member `d` of
  /// `terminal.hello`): the apps add the access token and a DPoP proof with the nonce of
  /// `hello`, the console adds no token (docs/spec/15-client-architecture.md 23.1).
  Future<Map<String, Object?>> identify(Map<String, Object?> hello);
}

/// One open terminal stream: text messages until `identify`, then only binary messages, each
/// one Noise message (docs/spec/24-terminal.md 5.2, 5.4).
abstract interface class TerminalChannel {
  /// Every received message: a [String] for a text message, a [Uint8List] for a binary one.
  /// The stream ends when the connection closes.
  Stream<Object> get messages;

  /// Sends a text message (only `terminal.identify`).
  void sendText(String text);

  /// Sends a binary message (one Noise message).
  void sendBinary(Uint8List bytes);

  /// Closes the connection with [code] (a normal close by default).
  Future<void> close([int code = 1000]);

  /// The close code the peer sent, null while open or when the connection broke without one.
  int? get closeCode;

  /// The close reason the peer sent (for `4100` the `close_reason` of the session), null when
  /// it sent none.
  String? get closeReason;
}

/// The pinned key of one host on this device (docs/spec/24-terminal.md 5.5).
final class HostKeyPin {
  /// Creates a pin.
  const HostKeyPin({
    required this.hostId,
    required this.noisePublicKey,
    required this.keyGeneration,
    required this.fingerprint,
    required this.firstSeen,
  });

  /// The host the pin belongs to.
  final String hostId;

  /// The static X25519 key of the host, base64url without padding as in `Host`.
  final String noisePublicKey;

  /// The `key_generation` of the host when the pin was written.
  final int keyGeneration;

  /// The `key_fingerprint` of the host when the pin was written.
  final String fingerprint;

  /// When this device first saw the key (after the first successful handshake).
  final DateTime firstSeen;
}

/// Stores the pinned host keys of this device: in the apps the encrypted table
/// `host_key_pins` of `local_store`, in the console IndexedDB of the origin (R-CL-15).
abstract interface class HostKeyPinStore {
  /// The pin of [hostId], or null when this device has none.
  Future<HostKeyPin?> read(String hostId);

  /// Stores [pin], replacing the pin of the same host.
  Future<void> write(HostKeyPin pin);
}

/// A change of a host or of one of the person's sessions that the view reacts to.
sealed class HostChange {
  const HostChange();
}

/// The host [hostId] changed (`host.updated`, `host.presence_changed`).
final class HostUpdated extends HostChange {
  /// Creates the change.
  const HostUpdated(this.hostId);

  /// The changed host.
  final String hostId;
}

/// The own session [sessionId] changed (`terminal_session.changed`).
final class TerminalSessionUpdated extends HostChange {
  /// Creates the change; [closeReason] is set when the session is closed.
  const TerminalSessionUpdated(
    this.sessionId, {
    this.closed = false,
    this.closeReason,
  });

  /// The changed session.
  final String sessionId;

  /// Whether the session is now `closed`.
  final bool closed;

  /// The `close_reason` of a closed session (docs/spec/24-terminal.md 10.3).
  final String? closeReason;
}

/// Changes to hosts and to the person's sessions: `sync_engine` in the apps, nothing in the
/// console (it reloads through REST; docs/spec/15-client-architecture.md 23.1).
abstract interface class HostEvents {
  /// The changes of the host [hostId] and of the sessions on it.
  Stream<HostChange> watch(String hostId);
}

/// Opens a hyperlink after the person confirmed it (docs/spec/24-terminal.md 11.2).
abstract interface class LinkOpener {
  /// Opens [target], an `https` or `http` URI the person confirmed.
  Future<void> open(Uri target);
}
