// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_backend_cost.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsBackendCost extends StatsBackendCost {
  @override
  final BillingMode billingMode;
  @override
  final String currency;
  @override
  final int amountMicros;
  @override
  final bool isEstimate;

  factory _$StatsBackendCost([
    void Function(StatsBackendCostBuilder)? updates,
  ]) => (StatsBackendCostBuilder()..update(updates))._build();

  _$StatsBackendCost._({
    required this.billingMode,
    required this.currency,
    required this.amountMicros,
    required this.isEstimate,
  }) : super._();
  @override
  StatsBackendCost rebuild(void Function(StatsBackendCostBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsBackendCostBuilder toBuilder() =>
      StatsBackendCostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsBackendCost &&
        billingMode == other.billingMode &&
        currency == other.currency &&
        amountMicros == other.amountMicros &&
        isEstimate == other.isEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, isEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsBackendCost')
          ..add('billingMode', billingMode)
          ..add('currency', currency)
          ..add('amountMicros', amountMicros)
          ..add('isEstimate', isEstimate))
        .toString();
  }
}

class StatsBackendCostBuilder
    implements Builder<StatsBackendCost, StatsBackendCostBuilder> {
  _$StatsBackendCost? _$v;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  bool? _isEstimate;
  bool? get isEstimate => _$this._isEstimate;
  set isEstimate(bool? isEstimate) => _$this._isEstimate = isEstimate;

  StatsBackendCostBuilder() {
    StatsBackendCost._defaults(this);
  }

  StatsBackendCostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _billingMode = $v.billingMode;
      _currency = $v.currency;
      _amountMicros = $v.amountMicros;
      _isEstimate = $v.isEstimate;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsBackendCost other) {
    _$v = other as _$StatsBackendCost;
  }

  @override
  void update(void Function(StatsBackendCostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsBackendCost build() => _build();

  _$StatsBackendCost _build() {
    final _$result =
        _$v ??
        _$StatsBackendCost._(
          billingMode: BuiltValueNullFieldError.checkNotNull(
            billingMode,
            r'StatsBackendCost',
            'billingMode',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'StatsBackendCost',
            'currency',
          ),
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'StatsBackendCost',
            'amountMicros',
          ),
          isEstimate: BuiltValueNullFieldError.checkNotNull(
            isEstimate,
            r'StatsBackendCost',
            'isEstimate',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
