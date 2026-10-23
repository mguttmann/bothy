// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_total.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostTotal extends CostTotal {
  @override
  final BillingMode billingMode;
  @override
  final int amountMicros;
  @override
  final String currency;
  @override
  final bool isEstimate;

  factory _$CostTotal([void Function(CostTotalBuilder)? updates]) =>
      (CostTotalBuilder()..update(updates))._build();

  _$CostTotal._({
    required this.billingMode,
    required this.amountMicros,
    required this.currency,
    required this.isEstimate,
  }) : super._();
  @override
  CostTotal rebuild(void Function(CostTotalBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostTotalBuilder toBuilder() => CostTotalBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostTotal &&
        billingMode == other.billingMode &&
        amountMicros == other.amountMicros &&
        currency == other.currency &&
        isEstimate == other.isEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, isEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostTotal')
          ..add('billingMode', billingMode)
          ..add('amountMicros', amountMicros)
          ..add('currency', currency)
          ..add('isEstimate', isEstimate))
        .toString();
  }
}

class CostTotalBuilder implements Builder<CostTotal, CostTotalBuilder> {
  _$CostTotal? _$v;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  bool? _isEstimate;
  bool? get isEstimate => _$this._isEstimate;
  set isEstimate(bool? isEstimate) => _$this._isEstimate = isEstimate;

  CostTotalBuilder() {
    CostTotal._defaults(this);
  }

  CostTotalBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _billingMode = $v.billingMode;
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _isEstimate = $v.isEstimate;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostTotal other) {
    _$v = other as _$CostTotal;
  }

  @override
  void update(void Function(CostTotalBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostTotal build() => _build();

  _$CostTotal _build() {
    final _$result =
        _$v ??
        _$CostTotal._(
          billingMode: BuiltValueNullFieldError.checkNotNull(
            billingMode,
            r'CostTotal',
            'billingMode',
          ),
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'CostTotal',
            'amountMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'CostTotal',
            'currency',
          ),
          isEstimate: BuiltValueNullFieldError.checkNotNull(
            isEstimate,
            r'CostTotal',
            'isEstimate',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
