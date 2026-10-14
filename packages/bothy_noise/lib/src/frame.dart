// SPDX-License-Identifier: Apache-2.0

// The frames of the terminal stream (docs/spec/24-terminal.md 5.4) with the byte order, body
// rules and directions of docs/DECISIONS.md D-M2-38-10, the same as services/host/internal/frame.

import 'dart:typed_data';

import 'bytes.dart';
import 'constants.dart';
import 'errors.dart';

/// The side that sent a frame.
enum FrameSender {
  /// The device of the person (desktop app, mobile app, web console).
  device,

  /// The host service.
  host,
}

const int _typeStart = 0x01;
const int _typeData = 0x02;
const int _typeResize = 0x03;
const int _typeClose = 0x04;
const int _typeExit = 0x05;
const int _typeReplayBegin = 0x06;
const int _typeReplayEnd = 0x07;
const int _maxSize = 0xffff;
const int _minInt32 = -0x80000000;
const int _maxInt32 = 0x7fffffff;

/// One frame of the terminal stream. [isInput] is true exactly for [DataFrame] and
/// [ResizeFrame], the frames that count as input for the idle timeout
/// (docs/spec/24-terminal.md 6.3).
sealed class TerminalFrame {
  const TerminalFrame();

  /// Whether the frame counts as input of the device for the idle timeout.
  bool get isInput => false;
}

/// `start` (0x01, device): the terminal size; once, as the first transport message.
final class StartFrame extends TerminalFrame {
  /// Creates a start frame; [cols] and [rows] must be 1 to 65535.
  const StartFrame(this.cols, this.rows);

  /// Columns, 1 to 65535.
  final int cols;

  /// Rows, 1 to 65535.
  final int rows;

  @override
  bool operator ==(Object other) =>
      other is StartFrame && other.cols == cols && other.rows == rows;

  @override
  int get hashCode => Object.hash(_typeStart, cols, rows);

  @override
  String toString() => 'StartFrame($cols x $rows)';
}

/// `data` (0x02, both directions): bytes of the PTY or input, 1 to 65518 of them.
final class DataFrame extends TerminalFrame {
  /// Creates a data frame; [bytes] is not copied.
  const DataFrame(this.bytes);

  /// The bytes of the PTY or of the input.
  final Uint8List bytes;

  @override
  bool get isInput => true;

  @override
  bool operator ==(Object other) {
    if (other is! DataFrame || other.bytes.length != bytes.length) return false;
    for (var i = 0; i < bytes.length; i++) {
      if (other.bytes[i] != bytes[i]) return false;
    }
    return true;
  }

  @override
  int get hashCode => Object.hash(_typeData, Object.hashAll(bytes));

  /// Names the length only, never the content.
  @override
  String toString() => 'DataFrame(${bytes.length} bytes)';
}

/// `resize` (0x03, device): the new terminal size.
final class ResizeFrame extends TerminalFrame {
  /// Creates a resize frame; [cols] and [rows] must be 1 to 65535.
  const ResizeFrame(this.cols, this.rows);

  /// Columns, 1 to 65535.
  final int cols;

  /// Rows, 1 to 65535.
  final int rows;

  @override
  bool get isInput => true;

  @override
  bool operator ==(Object other) =>
      other is ResizeFrame && other.cols == cols && other.rows == rows;

  @override
  int get hashCode => Object.hash(_typeResize, cols, rows);

  @override
  String toString() => 'ResizeFrame($cols x $rows)';
}

/// `close` (0x04, device): end the session. No body.
final class CloseFrame extends TerminalFrame {
  /// Creates the close frame.
  const CloseFrame();

  @override
  bool operator ==(Object other) => other is CloseFrame;

  @override
  int get hashCode => _typeClose;

  @override
  String toString() => 'CloseFrame()';
}

/// `exit` (0x05, host): the exit code of the process, a signed 32-bit integer.
final class ExitFrame extends TerminalFrame {
  /// Creates an exit frame; [code] must fit a signed 32-bit integer.
  const ExitFrame(this.code);

  /// The exit code of the process.
  final int code;

  @override
  bool operator ==(Object other) => other is ExitFrame && other.code == code;

  @override
  int get hashCode => Object.hash(_typeExit, code);

  @override
  String toString() => 'ExitFrame($code)';
}

/// `replay_begin` (0x06, host): the scrollback replay starts. No body.
final class ReplayBeginFrame extends TerminalFrame {
  /// Creates the frame.
  const ReplayBeginFrame();

  @override
  bool operator ==(Object other) => other is ReplayBeginFrame;

  @override
  int get hashCode => _typeReplayBegin;

  @override
  String toString() => 'ReplayBeginFrame()';
}

/// `replay_end` (0x07, host): the scrollback replay ends. No body.
final class ReplayEndFrame extends TerminalFrame {
  /// Creates the frame.
  const ReplayEndFrame();

  @override
  bool operator ==(Object other) => other is ReplayEndFrame;

  @override
  int get hashCode => _typeReplayEnd;

  @override
  String toString() => 'ReplayEndFrame()';
}

Never _malformed(int type, String detail) => throw FrameException(
  FrameError.malformed,
  'type 0x${type.toRadixString(16).padLeft(2, '0')}: $detail',
);

Uint8List _size(int type, int cols, int rows) {
  if (cols < 1 || cols > _maxSize || rows < 1 || rows > _maxSize) {
    _malformed(type, 'columns and rows must be 1 to 65535');
  }
  return Uint8List.fromList([
    type,
    cols >> 8,
    cols & 0xff,
    rows >> 8,
    rows & 0xff,
  ]);
}

/// Returns the bytes of [frame]. Throws [FrameException] with [FrameError.malformed] for a body
/// against the rules and [FrameError.tooLong] above [maxFrameLength] bytes.
Uint8List encodeFrame(TerminalFrame frame) {
  switch (frame) {
    case StartFrame(:final cols, :final rows):
      return _size(_typeStart, cols, rows);
    case ResizeFrame(:final cols, :final rows):
      return _size(_typeResize, cols, rows);
    case DataFrame(:final bytes):
      if (bytes.isEmpty) {
        _malformed(_typeData, 'no bytes');
      }
      if (1 + bytes.length > maxFrameLength) {
        throw FrameException(
          FrameError.tooLong,
          'type 0x02: ${1 + bytes.length} bytes, limit $maxFrameLength',
        );
      }
      return Uint8List(1 + bytes.length)
        ..[0] = _typeData
        ..setRange(1, 1 + bytes.length, bytes);
    case ExitFrame(:final code):
      if (code < _minInt32 || code > _maxInt32) {
        _malformed(_typeExit, 'exit code outside 32 bits');
      }
      final out = Uint8List(5)..[0] = _typeExit;
      ByteData.sublistView(out).setInt32(1, code);
      return out;
    case CloseFrame():
      return Uint8List.fromList(const [_typeClose]);
    case ReplayBeginFrame():
      return Uint8List.fromList(const [_typeReplayBegin]);
    case ReplayEndFrame():
      return Uint8List.fromList(const [_typeReplayEnd]);
  }
}

bool _allowed(int type, FrameSender from) => switch (type) {
  _typeStart || _typeResize || _typeClose => from == FrameSender.device,
  _typeExit || _typeReplayBegin || _typeReplayEnd => from == FrameSender.host,
  _ => true, // data
};

/// Parses one frame sent by [from]. For a [Uint8List] input the bytes of a [DataFrame] are a
/// view of it, else a copy.
///
/// Throws [FrameException] with [FrameError.empty], [FrameError.tooLong] (more than
/// [maxFrameLength] bytes), [FrameError.unknownType] (outside 0x01 to 0x07),
/// [FrameError.wrongDirection] or [FrameError.malformed]. Every one of them ends the session
/// (close code 4104); the text names the type byte and lengths only.
TerminalFrame decodeFrame(List<int> bytes, FrameSender from) {
  if (bytes.isEmpty) {
    throw const FrameException(FrameError.empty, 'no bytes');
  }
  if (bytes.length > maxFrameLength) {
    throw FrameException(
      FrameError.tooLong,
      '${bytes.length} bytes, limit $maxFrameLength',
    );
  }
  if (!isByteList(bytes)) {
    // Uint8List.fromList would truncate such values silently; the Go side cannot get them.
    throw const FrameException(FrameError.malformed, 'value outside 0 to 255');
  }
  final b = bytes is Uint8List ? bytes : Uint8List.fromList(bytes);
  final type = b[0];
  final typeText = 'type 0x${type.toRadixString(16).padLeft(2, '0')}';
  if (type < _typeStart || type > _typeReplayEnd) {
    throw FrameException(FrameError.unknownType, typeText);
  }
  if (!_allowed(type, from)) {
    throw FrameException(
      FrameError.wrongDirection,
      '$typeText from ${from.name}',
    );
  }
  final bodyLength = b.length - 1;
  final view = ByteData.sublistView(b);
  switch (type) {
    case _typeStart || _typeResize:
      if (bodyLength != 4) {
        _malformed(type, 'body of $bodyLength bytes, want 4');
      }
      final cols = view.getUint16(1);
      final rows = view.getUint16(3);
      if (cols == 0 || rows == 0) {
        _malformed(type, 'columns and rows must be 1 to 65535');
      }
      return type == _typeStart
          ? StartFrame(cols, rows)
          : ResizeFrame(cols, rows);
    case _typeData:
      if (bodyLength == 0) {
        _malformed(type, 'no bytes');
      }
      return DataFrame(Uint8List.sublistView(b, 1));
    case _typeExit:
      if (bodyLength != 4) {
        _malformed(type, 'body of $bodyLength bytes, want 4');
      }
      return ExitFrame(view.getInt32(1));
    default:
      if (bodyLength != 0) {
        _malformed(type, 'body of $bodyLength bytes, want 0');
      }
      return switch (type) {
        _typeClose => const CloseFrame(),
        _typeReplayBegin => const ReplayBeginFrame(),
        _ => const ReplayEndFrame(),
      };
  }
}
