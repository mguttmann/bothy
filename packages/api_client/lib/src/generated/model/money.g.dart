// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'money.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Money extends Money {
  @override
  final int amountMicros;
  @override
  final String currency;
  @override
  final bool isEstimate;

  factory _$Money([void Function(MoneyBuilder)? updates]) =>
      (MoneyBuilder()..update(updates))._build();

  _$Money._({
    required this.amountMicros,
    required this.currency,
    required this.isEstimate,
  }) : super._();
  @override
  Money rebuild(void Function(MoneyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MoneyBuilder toBuilder() => MoneyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Money &&
        amountMicros == other.amountMicros &&
        currency == other.currency &&
        isEstimate == other.isEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, isEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Money')
          ..add('amountMicros', amountMicros)
          ..add('currency', currency)
          ..add('isEstimate', isEstimate))
        .toString();
  }
}

class MoneyBuilder implements Builder<Money, MoneyBuilder> {
  _$Money? _$v;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  bool? _isEstimate;
  bool? get isEstimate => _$this._isEstimate;
  set isEstimate(bool? isEstimate) => _$this._isEstimate = isEstimate;

  MoneyBuilder() {
    Money._defaults(this);
  }

  MoneyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _isEstimate = $v.isEstimate;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Money other) {
    _$v = other as _$Money;
  }

  @override
  void update(void Function(MoneyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Money build() => _build();

  _$Money _build() {
    final _$result =
        _$v ??
        _$Money._(
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'Money',
            'amountMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'Money',
            'currency',
          ),
          isEstimate: BuiltValueNullFieldError.checkNotNull(
            isEstimate,
            r'Money',
            'isEstimate',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
