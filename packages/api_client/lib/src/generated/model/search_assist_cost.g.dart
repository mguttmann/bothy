// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_cost.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistCost extends SearchAssistCost {
  @override
  final int amountMicros;
  @override
  final String currency;

  factory _$SearchAssistCost([
    void Function(SearchAssistCostBuilder)? updates,
  ]) => (SearchAssistCostBuilder()..update(updates))._build();

  _$SearchAssistCost._({required this.amountMicros, required this.currency})
    : super._();
  @override
  SearchAssistCost rebuild(void Function(SearchAssistCostBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchAssistCostBuilder toBuilder() =>
      SearchAssistCostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistCost &&
        amountMicros == other.amountMicros &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistCost')
          ..add('amountMicros', amountMicros)
          ..add('currency', currency))
        .toString();
  }
}

class SearchAssistCostBuilder
    implements Builder<SearchAssistCost, SearchAssistCostBuilder> {
  _$SearchAssistCost? _$v;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  SearchAssistCostBuilder() {
    SearchAssistCost._defaults(this);
  }

  SearchAssistCostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistCost other) {
    _$v = other as _$SearchAssistCost;
  }

  @override
  void update(void Function(SearchAssistCostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistCost build() => _build();

  _$SearchAssistCost _build() {
    final _$result =
        _$v ??
        _$SearchAssistCost._(
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'SearchAssistCost',
            'amountMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'SearchAssistCost',
            'currency',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
