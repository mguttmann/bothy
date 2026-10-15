// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'operator_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OperatorSet extends OperatorSet {
  @override
  final OperatorMode mode;
  @override
  final String? agentId;
  @override
  final String reason;

  factory _$OperatorSet([void Function(OperatorSetBuilder)? updates]) =>
      (OperatorSetBuilder()..update(updates))._build();

  _$OperatorSet._({required this.mode, this.agentId, required this.reason})
    : super._();
  @override
  OperatorSet rebuild(void Function(OperatorSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OperatorSetBuilder toBuilder() => OperatorSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OperatorSet &&
        mode == other.mode &&
        agentId == other.agentId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'OperatorSet')
          ..add('mode', mode)
          ..add('agentId', agentId)
          ..add('reason', reason))
        .toString();
  }
}

class OperatorSetBuilder implements Builder<OperatorSet, OperatorSetBuilder> {
  _$OperatorSet? _$v;

  OperatorMode? _mode;
  OperatorMode? get mode => _$this._mode;
  set mode(OperatorMode? mode) => _$this._mode = mode;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  OperatorSetBuilder() {
    OperatorSet._defaults(this);
  }

  OperatorSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _mode = $v.mode;
      _agentId = $v.agentId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OperatorSet other) {
    _$v = other as _$OperatorSet;
  }

  @override
  void update(void Function(OperatorSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OperatorSet build() => _build();

  _$OperatorSet _build() {
    final _$result =
        _$v ??
        _$OperatorSet._(
          mode: BuiltValueNullFieldError.checkNotNull(
            mode,
            r'OperatorSet',
            'mode',
          ),
          agentId: agentId,
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'OperatorSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
