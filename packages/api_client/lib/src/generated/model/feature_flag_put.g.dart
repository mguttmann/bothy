// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'feature_flag_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FeatureFlagPut extends FeatureFlagPut {
  @override
  final bool enabled;
  @override
  final FlagRollout? rollout;
  @override
  final String reason;

  factory _$FeatureFlagPut([void Function(FeatureFlagPutBuilder)? updates]) =>
      (FeatureFlagPutBuilder()..update(updates))._build();

  _$FeatureFlagPut._({
    required this.enabled,
    this.rollout,
    required this.reason,
  }) : super._();
  @override
  FeatureFlagPut rebuild(void Function(FeatureFlagPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FeatureFlagPutBuilder toBuilder() => FeatureFlagPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FeatureFlagPut &&
        enabled == other.enabled &&
        rollout == other.rollout &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, rollout.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'FeatureFlagPut')
          ..add('enabled', enabled)
          ..add('rollout', rollout)
          ..add('reason', reason))
        .toString();
  }
}

class FeatureFlagPutBuilder
    implements Builder<FeatureFlagPut, FeatureFlagPutBuilder> {
  _$FeatureFlagPut? _$v;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  FlagRolloutBuilder? _rollout;
  FlagRolloutBuilder get rollout => _$this._rollout ??= FlagRolloutBuilder();
  set rollout(FlagRolloutBuilder? rollout) => _$this._rollout = rollout;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  FeatureFlagPutBuilder() {
    FeatureFlagPut._defaults(this);
  }

  FeatureFlagPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _enabled = $v.enabled;
      _rollout = $v.rollout?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FeatureFlagPut other) {
    _$v = other as _$FeatureFlagPut;
  }

  @override
  void update(void Function(FeatureFlagPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FeatureFlagPut build() => _build();

  _$FeatureFlagPut _build() {
    _$FeatureFlagPut _$result;
    try {
      _$result =
          _$v ??
          _$FeatureFlagPut._(
            enabled: BuiltValueNullFieldError.checkNotNull(
              enabled,
              r'FeatureFlagPut',
              'enabled',
            ),
            rollout: _rollout?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'FeatureFlagPut',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'rollout';
        _rollout?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'FeatureFlagPut',
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
