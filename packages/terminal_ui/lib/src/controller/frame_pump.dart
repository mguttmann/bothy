// SPDX-License-Identifier: Apache-2.0

/// Encrypts, encodes, decrypts and decodes the frames of one stream after the handshake
/// (docs/spec/24-terminal.md 5.4), and coalesces size changes to at most one `resize` per
/// frame tick (docs/spec/15-client-architecture.md 23.2 item 5).
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';

/// Schedules [callback] once at the next frame tick. The view passes the scheduler of Flutter;
/// tests pass their own tick.
typedef FrameTickScheduler = void Function(void Function() callback);

/// The largest input of one `data` frame: the largest frame minus its type byte.
const int maxDataFrameBytes = maxFrameLength - 1;

/// One stream's frame layer over a [NoiseTransport].
///
/// Sends run strictly in order, because every message uses the next nonce of the transport;
/// a failed encryption or a failed send breaks the pump ([onError]) and the controller ends
/// the stream with a hard warning (Noise errors are tampering, docs/spec/24-terminal.md 5.2).
final class FramePump {
  /// Creates the pump; [send] puts one Noise message on the stream, [scheduleTick] runs the
  /// coalesced resize, [onError] receives the first error of a send.
  FramePump({
    required this._transport,
    required this._send,
    required this._scheduleTick,
    required this._onError,
  });

  final NoiseTransport _transport;
  final void Function(Uint8List message) _send;
  final FrameTickScheduler _scheduleTick;
  final void Function(Object error) _onError;
  Future<void> _tail = Future<void>.value();
  (int, int)? _pendingResize;
  bool _tickScheduled = false;
  bool _broken = false;

  /// Sends [frame] after every frame sent before it.
  Future<void> sendFrame(TerminalFrame frame) {
    final next = _tail.then((_) async {
      if (_broken) return;
      try {
        final message = await _transport.encrypt(encodeFrame(frame));
        _send(message);
      } on Object catch (e) {
        _broken = true;
        _onError(e);
      }
    });
    _tail = next;
    return next;
  }

  /// Sends [bytes] of input as `data` frames, split at [maxDataFrameBytes]; nothing for an
  /// empty input (an empty `data` frame is not allowed, 24 5.4). Every chunk is queued at once,
  /// so input sent while the chunks are on their way follows the whole input.
  Future<void> sendInput(List<int> bytes) {
    var last = _tail;
    for (var start = 0; start < bytes.length; start += maxDataFrameBytes) {
      final end = start + maxDataFrameBytes < bytes.length
          ? start + maxDataFrameBytes
          : bytes.length;
      last = sendFrame(
        DataFrame(Uint8List.fromList(bytes.sublist(start, end))),
      );
    }
    return last;
  }

  /// Records a new size; the last size within one frame tick is sent as one `resize`.
  void requestResize(int cols, int rows) {
    _pendingResize = (cols, rows);
    if (_tickScheduled) return;
    _tickScheduled = true;
    _scheduleTick(_flushResize);
  }

  void _flushResize() {
    _tickScheduled = false;
    if (_broken) return;
    final size = _pendingResize;
    _pendingResize = null;
    if (size == null) return;
    unawaited(sendFrame(ResizeFrame(size.$1, size.$2)));
  }

  /// Decrypts and decodes one message of the host. Throws [NoiseException] or
  /// [FrameException]; both end the session with a hard warning.
  Future<TerminalFrame> receive(List<int> message) async =>
      decodeFrame(await _transport.decrypt(message), FrameSender.host);

  /// Stops the pump: queued and later frames, and a pending resize tick, are dropped. The
  /// controller stops the pump of a stream it closes.
  void stop() {
    _broken = true;
    _pendingResize = null;
  }

  /// Completes when every frame queued so far was handed to the stream.
  Future<void> get drained => _tail;
}
