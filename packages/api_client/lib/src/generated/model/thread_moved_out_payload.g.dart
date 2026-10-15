// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_moved_out_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadMovedOutPayload extends ThreadMovedOutPayload {
  @override
  final String threadId;
  @override
  final String channelId;

  factory _$ThreadMovedOutPayload([
    void Function(ThreadMovedOutPayloadBuilder)? updates,
  ]) => (ThreadMovedOutPayloadBuilder()..update(updates))._build();

  _$ThreadMovedOutPayload._({required this.threadId, required this.channelId})
    : super._();
  @override
  ThreadMovedOutPayload rebuild(
    void Function(ThreadMovedOutPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ThreadMovedOutPayloadBuilder toBuilder() =>
      ThreadMovedOutPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadMovedOutPayload &&
        threadId == other.threadId &&
        channelId == other.channelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadMovedOutPayload')
          ..add('threadId', threadId)
          ..add('channelId', channelId))
        .toString();
  }
}

class ThreadMovedOutPayloadBuilder
    implements Builder<ThreadMovedOutPayload, ThreadMovedOutPayloadBuilder> {
  _$ThreadMovedOutPayload? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  ThreadMovedOutPayloadBuilder() {
    ThreadMovedOutPayload._defaults(this);
  }

  ThreadMovedOutPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadMovedOutPayload other) {
    _$v = other as _$ThreadMovedOutPayload;
  }

  @override
  void update(void Function(ThreadMovedOutPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadMovedOutPayload build() => _build();

  _$ThreadMovedOutPayload _build() {
    final _$result =
        _$v ??
        _$ThreadMovedOutPayload._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'ThreadMovedOutPayload',
            'threadId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ThreadMovedOutPayload',
            'channelId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
