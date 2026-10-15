// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_move.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadMove extends ThreadMove {
  @override
  final String targetChannelId;
  @override
  final String reason;

  factory _$ThreadMove([void Function(ThreadMoveBuilder)? updates]) =>
      (ThreadMoveBuilder()..update(updates))._build();

  _$ThreadMove._({required this.targetChannelId, required this.reason})
    : super._();
  @override
  ThreadMove rebuild(void Function(ThreadMoveBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadMoveBuilder toBuilder() => ThreadMoveBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadMove &&
        targetChannelId == other.targetChannelId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, targetChannelId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadMove')
          ..add('targetChannelId', targetChannelId)
          ..add('reason', reason))
        .toString();
  }
}

class ThreadMoveBuilder implements Builder<ThreadMove, ThreadMoveBuilder> {
  _$ThreadMove? _$v;

  String? _targetChannelId;
  String? get targetChannelId => _$this._targetChannelId;
  set targetChannelId(String? targetChannelId) =>
      _$this._targetChannelId = targetChannelId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ThreadMoveBuilder() {
    ThreadMove._defaults(this);
  }

  ThreadMoveBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _targetChannelId = $v.targetChannelId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadMove other) {
    _$v = other as _$ThreadMove;
  }

  @override
  void update(void Function(ThreadMoveBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadMove build() => _build();

  _$ThreadMove _build() {
    final _$result =
        _$v ??
        _$ThreadMove._(
          targetChannelId: BuiltValueNullFieldError.checkNotNull(
            targetChannelId,
            r'ThreadMove',
            'targetChannelId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'ThreadMove',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
