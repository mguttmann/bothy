// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_moved_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadMovedPayload extends ThreadMovedPayload {
  @override
  final Thread thread;
  @override
  final String fromChannelId;

  factory _$ThreadMovedPayload([
    void Function(ThreadMovedPayloadBuilder)? updates,
  ]) => (ThreadMovedPayloadBuilder()..update(updates))._build();

  _$ThreadMovedPayload._({required this.thread, required this.fromChannelId})
    : super._();
  @override
  ThreadMovedPayload rebuild(
    void Function(ThreadMovedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ThreadMovedPayloadBuilder toBuilder() =>
      ThreadMovedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadMovedPayload &&
        thread == other.thread &&
        fromChannelId == other.fromChannelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, thread.hashCode);
    _$hash = $jc(_$hash, fromChannelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadMovedPayload')
          ..add('thread', thread)
          ..add('fromChannelId', fromChannelId))
        .toString();
  }
}

class ThreadMovedPayloadBuilder
    implements Builder<ThreadMovedPayload, ThreadMovedPayloadBuilder> {
  _$ThreadMovedPayload? _$v;

  ThreadBuilder? _thread;
  ThreadBuilder get thread => _$this._thread ??= ThreadBuilder();
  set thread(ThreadBuilder? thread) => _$this._thread = thread;

  String? _fromChannelId;
  String? get fromChannelId => _$this._fromChannelId;
  set fromChannelId(String? fromChannelId) =>
      _$this._fromChannelId = fromChannelId;

  ThreadMovedPayloadBuilder() {
    ThreadMovedPayload._defaults(this);
  }

  ThreadMovedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _thread = $v.thread.toBuilder();
      _fromChannelId = $v.fromChannelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadMovedPayload other) {
    _$v = other as _$ThreadMovedPayload;
  }

  @override
  void update(void Function(ThreadMovedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadMovedPayload build() => _build();

  _$ThreadMovedPayload _build() {
    _$ThreadMovedPayload _$result;
    try {
      _$result =
          _$v ??
          _$ThreadMovedPayload._(
            thread: thread.build(),
            fromChannelId: BuiltValueNullFieldError.checkNotNull(
              fromChannelId,
              r'ThreadMovedPayload',
              'fromChannelId',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'thread';
        thread.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadMovedPayload',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
