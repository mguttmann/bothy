// SPDX-License-Identifier: Apache-2.0

/// Gateway frames `{op, d}` (04 2.3, 04 6.2) over the generated models of `api_client`
/// (brief M4-13 Design note 1).
library;

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:built_value/serializer.dart';

/// One frame of the gateway, wrapping the generated model of its op.
///
/// Only the ops a person's client sends or receives have a class (04 6.2); agent ops,
/// `stats.*` (M6) and ops of a later minor version are ignored by [decodeFrame], because
/// changes within a protocol version are additive (04 6.1).
///
/// Example:
/// ```dart
/// final text = encodeFrame(HeartbeatFrame(GatewayHeartbeat((b) => b..lastSeq = 41)));
/// final frame = decodeFrame(text); // HeartbeatFrame
/// ```
sealed class GatewayFrame {
  const GatewayFrame();

  /// The op name of 04 6.2, for example `identify`.
  String get op;

  /// The JSON of `d`, without `null` members (04 2.2).
  Object? toData();
}

/// A frame whose `d` is a generated model of type [T].
sealed class ModelFrame<T extends Object> extends GatewayFrame {
  const ModelFrame(this.model);

  /// The generated model of `d`.
  final T model;

  @override
  Object? toData() => _dropNulls(
    standardSerializers.serialize(model, specifiedType: FullType(T)),
  );
}

/// `identify`: login of a new gateway session (04 6.3 item 3).
final class IdentifyFrame extends ModelFrame<GatewayIdentify> {
  /// Wraps [model].
  const IdentifyFrame(super.model);
  @override
  String get op => 'identify';
}

/// `resume`: resuming a gateway session within `resume_window_ms` (04 6.3 item 7).
final class ResumeFrame extends ModelFrame<GatewayResume> {
  /// Wraps [model].
  const ResumeFrame(super.model);
  @override
  String get op => 'resume';
}

/// `heartbeat` with the last processed `seq` (04 6.3 item 5).
final class HeartbeatFrame extends ModelFrame<GatewayHeartbeat> {
  /// Wraps [model].
  const HeartbeatFrame(super.model);
  @override
  String get op => 'heartbeat';
}

/// `reauth`: a new access token during the connection (04 6.3 item 6).
final class ReauthFrame extends ModelFrame<GatewayReauth> {
  /// Wraps [model].
  const ReauthFrame(super.model);
  @override
  String get op => 'reauth';
}

/// `focus`: the complete set of visible threads with `idle` (04 6.2, 11.3 item 2).
final class FocusFrame extends ModelFrame<GatewayFocus> {
  /// Wraps [model].
  const FocusFrame(super.model);
  @override
  String get op => 'focus';
}

/// `typing.start` (04 12 item 1).
final class TypingStartFrame extends ModelFrame<GatewayTyping> {
  /// Wraps [model].
  const TypingStartFrame(super.model);
  @override
  String get op => 'typing.start';
}

/// `typing.stop`: resets the throttle of the connection (04 12 item 1).
final class TypingStopFrame extends ModelFrame<GatewayTyping> {
  /// Wraps [model].
  const TypingStopFrame(super.model);
  @override
  String get op => 'typing.stop';
}

/// `hello`: the first frame of the server (04 6.3 item 2).
final class HelloFrame extends ModelFrame<GatewayHello> {
  /// Wraps [model].
  const HelloFrame(super.model);
  @override
  String get op => 'hello';
}

/// `ready`: the session is ready, with a snapshot (04 6.3 item 4).
final class ReadyFrame extends ModelFrame<GatewayReady> {
  /// Wraps [model].
  const ReadyFrame(super.model);
  @override
  String get op => 'ready';
}

/// `resumed`: the resumption is confirmed (04 6.3 item 7).
final class ResumedFrame extends ModelFrame<GatewayResumed> {
  /// Wraps [model].
  const ResumedFrame(super.model);
  @override
  String get op => 'resumed';
}

/// `dispatch`: one event; the raw envelope map is kept and decoded with `decodeEnvelope` of
/// `api_client` only by the dispatcher of M4-14, so an unknown event type never breaks the
/// connection.
final class DispatchFrame extends GatewayFrame {
  /// Wraps the raw [envelope].
  const DispatchFrame(this.envelope);

  /// The envelope as received (04 2.2).
  final Map<String, Object?> envelope;

  @override
  String get op => 'dispatch';

  @override
  Object? toData() => _dropNulls(envelope);
}

/// `cursor`: advance the cursor without an event (04 6.2).
final class CursorFrame extends ModelFrame<GatewayCursor> {
  /// Wraps [model].
  const CursorFrame(super.model);
  @override
  String get op => 'cursor';
}

/// `catchup.complete`: catch-up finished, live from now on (04 7.2).
final class CatchupCompleteFrame extends ModelFrame<GatewayCatchupComplete> {
  /// Wraps [model].
  const CatchupCompleteFrame(super.model);
  @override
  String get op => 'catchup.complete';
}

/// `resync_required`: catch-up not possible (04 7.3).
final class ResyncRequiredFrame extends ModelFrame<GatewayResyncRequired> {
  /// Wraps [model].
  const ResyncRequiredFrame(super.model);
  @override
  String get op => 'resync_required';
}

/// `heartbeat_ack` (04 6.3 item 5).
final class HeartbeatAckFrame extends ModelFrame<GatewayHeartbeatAck> {
  /// Wraps [model].
  const HeartbeatAckFrame(super.model);
  @override
  String get op => 'heartbeat_ack';
}

/// `reauth_required`: the access token expires soon (04 6.3 item 6).
final class ReauthRequiredFrame extends ModelFrame<GatewayReauthRequired> {
  /// Wraps [model].
  const ReauthRequiredFrame(super.model);
  @override
  String get op => 'reauth_required';
}

/// `error`: an op failed; the connection stays (04 6.2).
final class ErrorFrame extends ModelFrame<GatewayError> {
  /// Wraps [model].
  const ErrorFrame(super.model);
  @override
  String get op => 'error';
}

/// A text frame that is not valid JSON, has no `op`, or whose `d` does not fit the schema of
/// its op.
///
/// The message names only the cause, never the frame content (15 16), because a frame can
/// carry tokens or message text.
final class FrameDecodeError implements Exception {
  /// Creates the error for [cause].
  const FrameDecodeError(this.cause);

  /// What was wrong, without content.
  final String cause;

  @override
  String toString() => 'FrameDecodeError($cause)';
}

/// Encodes [frame] as the text of a WebSocket frame `{"op": ..., "d": ...}` without `null`
/// members (04 2.2, 2.3).
String encodeFrame(GatewayFrame frame) =>
    jsonEncode({'op': frame.op, 'd': frame.toData()});

typedef _Decoder = GatewayFrame Function(Object? d);

GatewayFrame Function(Object? d) _model<T extends Object>(
  GatewayFrame Function(T model) wrap,
) =>
    (d) => wrap(
      standardSerializers.deserialize(d, specifiedType: FullType(T)) as T,
    );

final Map<String, _Decoder> _decoders = {
  'identify': _model<GatewayIdentify>(IdentifyFrame.new),
  'resume': _model<GatewayResume>(ResumeFrame.new),
  'heartbeat': _model<GatewayHeartbeat>(HeartbeatFrame.new),
  'reauth': _model<GatewayReauth>(ReauthFrame.new),
  'focus': _model<GatewayFocus>(FocusFrame.new),
  'typing.start': _model<GatewayTyping>(TypingStartFrame.new),
  'typing.stop': _model<GatewayTyping>(TypingStopFrame.new),
  'hello': _model<GatewayHello>(HelloFrame.new),
  'ready': _model<GatewayReady>(ReadyFrame.new),
  'resumed': _model<GatewayResumed>(ResumedFrame.new),
  'dispatch': (d) {
    if (d is! Map<String, Object?>) {
      throw const FrameDecodeError('dispatch without an envelope object');
    }
    return DispatchFrame(d);
  },
  'cursor': _model<GatewayCursor>(CursorFrame.new),
  'catchup.complete': _model<GatewayCatchupComplete>(CatchupCompleteFrame.new),
  'resync_required': _model<GatewayResyncRequired>(ResyncRequiredFrame.new),
  'heartbeat_ack': _model<GatewayHeartbeatAck>(HeartbeatAckFrame.new),
  'reauth_required': _model<GatewayReauthRequired>(ReauthRequiredFrame.new),
  'error': _model<GatewayError>(ErrorFrame.new),
};

/// Decodes one text frame.
///
/// Returns `null` for an op this client does not know (04 6.1: ignored); unknown members of a
/// known op are ignored by the generated serializers. Throws [FrameDecodeError] for text that
/// is not a JSON object with a string `op`, or whose `d` does not fit the schema of the op.
GatewayFrame? decodeFrame(String text) {
  final Object? json;
  try {
    json = jsonDecode(text);
  } on FormatException {
    throw const FrameDecodeError('not JSON');
  }
  if (json is! Map<String, Object?>) {
    throw const FrameDecodeError('not a JSON object');
  }
  final op = json['op'];
  if (op is! String) throw const FrameDecodeError('no op');
  final decoder = _decoders[op];
  if (decoder == null) return null;
  try {
    return decoder(json['d']);
  } on FrameDecodeError {
    rethrow;
  } on Object {
    // The serializers throw various error types for a wrong shape; only the op is named.
    throw FrameDecodeError('d of $op does not fit its schema');
  }
}

/// [value] without `null` members of maps, recursively (04 2.2: outgoing JSON has no `null`).
Object? _dropNulls(Object? value) => switch (value) {
  Map() => {
    for (final e in value.entries)
      if (e.value != null) e.key as String: _dropNulls(e.value),
  },
  List() => [for (final v in value) _dropNulls(v)],
  _ => value,
};
