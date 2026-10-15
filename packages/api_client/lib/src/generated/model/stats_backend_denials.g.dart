// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_backend_denials.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsBackendDenials extends StatsBackendDenials {
  @override
  final int i2Denied;
  @override
  final int billingModeMismatch;

  factory _$StatsBackendDenials([
    void Function(StatsBackendDenialsBuilder)? updates,
  ]) => (StatsBackendDenialsBuilder()..update(updates))._build();

  _$StatsBackendDenials._({
    required this.i2Denied,
    required this.billingModeMismatch,
  }) : super._();
  @override
  StatsBackendDenials rebuild(
    void Function(StatsBackendDenialsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatsBackendDenialsBuilder toBuilder() =>
      StatsBackendDenialsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsBackendDenials &&
        i2Denied == other.i2Denied &&
        billingModeMismatch == other.billingModeMismatch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, i2Denied.hashCode);
    _$hash = $jc(_$hash, billingModeMismatch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsBackendDenials')
          ..add('i2Denied', i2Denied)
          ..add('billingModeMismatch', billingModeMismatch))
        .toString();
  }
}

class StatsBackendDenialsBuilder
    implements Builder<StatsBackendDenials, StatsBackendDenialsBuilder> {
  _$StatsBackendDenials? _$v;

  int? _i2Denied;
  int? get i2Denied => _$this._i2Denied;
  set i2Denied(int? i2Denied) => _$this._i2Denied = i2Denied;

  int? _billingModeMismatch;
  int? get billingModeMismatch => _$this._billingModeMismatch;
  set billingModeMismatch(int? billingModeMismatch) =>
      _$this._billingModeMismatch = billingModeMismatch;

  StatsBackendDenialsBuilder() {
    StatsBackendDenials._defaults(this);
  }

  StatsBackendDenialsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _i2Denied = $v.i2Denied;
      _billingModeMismatch = $v.billingModeMismatch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsBackendDenials other) {
    _$v = other as _$StatsBackendDenials;
  }

  @override
  void update(void Function(StatsBackendDenialsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsBackendDenials build() => _build();

  _$StatsBackendDenials _build() {
    final _$result =
        _$v ??
        _$StatsBackendDenials._(
          i2Denied: BuiltValueNullFieldError.checkNotNull(
            i2Denied,
            r'StatsBackendDenials',
            'i2Denied',
          ),
          billingModeMismatch: BuiltValueNullFieldError.checkNotNull(
            billingModeMismatch,
            r'StatsBackendDenials',
            'billingModeMismatch',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
