// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadPayload extends ThreadPayload {
  @override
  final Thread thread;

  factory _$ThreadPayload([void Function(ThreadPayloadBuilder)? updates]) =>
      (ThreadPayloadBuilder()..update(updates))._build();

  _$ThreadPayload._({required this.thread}) : super._();
  @override
  ThreadPayload rebuild(void Function(ThreadPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadPayloadBuilder toBuilder() => ThreadPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadPayload && thread == other.thread;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, thread.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ThreadPayload',
    )..add('thread', thread)).toString();
  }
}

class ThreadPayloadBuilder
    implements Builder<ThreadPayload, ThreadPayloadBuilder> {
  _$ThreadPayload? _$v;

  ThreadBuilder? _thread;
  ThreadBuilder get thread => _$this._thread ??= ThreadBuilder();
  set thread(ThreadBuilder? thread) => _$this._thread = thread;

  ThreadPayloadBuilder() {
    ThreadPayload._defaults(this);
  }

  ThreadPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _thread = $v.thread.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadPayload other) {
    _$v = other as _$ThreadPayload;
  }

  @override
  void update(void Function(ThreadPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadPayload build() => _build();

  _$ThreadPayload _build() {
    _$ThreadPayload _$result;
    try {
      _$result = _$v ?? _$ThreadPayload._(thread: thread.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'thread';
        thread.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadPayload',
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
