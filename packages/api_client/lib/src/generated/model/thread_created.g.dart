// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadCreated extends ThreadCreated {
  @override
  final Thread thread;
  @override
  final Message? firstMessage;

  factory _$ThreadCreated([void Function(ThreadCreatedBuilder)? updates]) =>
      (ThreadCreatedBuilder()..update(updates))._build();

  _$ThreadCreated._({required this.thread, this.firstMessage}) : super._();
  @override
  ThreadCreated rebuild(void Function(ThreadCreatedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadCreatedBuilder toBuilder() => ThreadCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadCreated &&
        thread == other.thread &&
        firstMessage == other.firstMessage;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, thread.hashCode);
    _$hash = $jc(_$hash, firstMessage.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadCreated')
          ..add('thread', thread)
          ..add('firstMessage', firstMessage))
        .toString();
  }
}

class ThreadCreatedBuilder
    implements Builder<ThreadCreated, ThreadCreatedBuilder> {
  _$ThreadCreated? _$v;

  ThreadBuilder? _thread;
  ThreadBuilder get thread => _$this._thread ??= ThreadBuilder();
  set thread(ThreadBuilder? thread) => _$this._thread = thread;

  MessageBuilder? _firstMessage;
  MessageBuilder get firstMessage => _$this._firstMessage ??= MessageBuilder();
  set firstMessage(MessageBuilder? firstMessage) =>
      _$this._firstMessage = firstMessage;

  ThreadCreatedBuilder() {
    ThreadCreated._defaults(this);
  }

  ThreadCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _thread = $v.thread.toBuilder();
      _firstMessage = $v.firstMessage?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadCreated other) {
    _$v = other as _$ThreadCreated;
  }

  @override
  void update(void Function(ThreadCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadCreated build() => _build();

  _$ThreadCreated _build() {
    _$ThreadCreated _$result;
    try {
      _$result =
          _$v ??
          _$ThreadCreated._(
            thread: thread.build(),
            firstMessage: _firstMessage?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'thread';
        thread.build();
        _$failedField = 'firstMessage';
        _firstMessage?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadCreated',
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
