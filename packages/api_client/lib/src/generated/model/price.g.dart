// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'price.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Price extends Price {
  @override
  final String id;
  @override
  final String provider;
  @override
  final String model;
  @override
  final PriceUnit unit;
  @override
  final int priceMicros;
  @override
  final String currency;
  @override
  final DateTime validFrom;
  @override
  final DateTime? validTo;
  @override
  final String sourceUrl;
  @override
  final DateTime retrievedAt;

  factory _$Price([void Function(PriceBuilder)? updates]) =>
      (PriceBuilder()..update(updates))._build();

  _$Price._({
    required this.id,
    required this.provider,
    required this.model,
    required this.unit,
    required this.priceMicros,
    required this.currency,
    required this.validFrom,
    this.validTo,
    required this.sourceUrl,
    required this.retrievedAt,
  }) : super._();
  @override
  Price rebuild(void Function(PriceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PriceBuilder toBuilder() => PriceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Price &&
        id == other.id &&
        provider == other.provider &&
        model == other.model &&
        unit == other.unit &&
        priceMicros == other.priceMicros &&
        currency == other.currency &&
        validFrom == other.validFrom &&
        validTo == other.validTo &&
        sourceUrl == other.sourceUrl &&
        retrievedAt == other.retrievedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, unit.hashCode);
    _$hash = $jc(_$hash, priceMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, validFrom.hashCode);
    _$hash = $jc(_$hash, validTo.hashCode);
    _$hash = $jc(_$hash, sourceUrl.hashCode);
    _$hash = $jc(_$hash, retrievedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Price')
          ..add('id', id)
          ..add('provider', provider)
          ..add('model', model)
          ..add('unit', unit)
          ..add('priceMicros', priceMicros)
          ..add('currency', currency)
          ..add('validFrom', validFrom)
          ..add('validTo', validTo)
          ..add('sourceUrl', sourceUrl)
          ..add('retrievedAt', retrievedAt))
        .toString();
  }
}

class PriceBuilder implements Builder<Price, PriceBuilder> {
  _$Price? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _provider;
  String? get provider => _$this._provider;
  set provider(String? provider) => _$this._provider = provider;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  PriceUnit? _unit;
  PriceUnit? get unit => _$this._unit;
  set unit(PriceUnit? unit) => _$this._unit = unit;

  int? _priceMicros;
  int? get priceMicros => _$this._priceMicros;
  set priceMicros(int? priceMicros) => _$this._priceMicros = priceMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  DateTime? _validFrom;
  DateTime? get validFrom => _$this._validFrom;
  set validFrom(DateTime? validFrom) => _$this._validFrom = validFrom;

  DateTime? _validTo;
  DateTime? get validTo => _$this._validTo;
  set validTo(DateTime? validTo) => _$this._validTo = validTo;

  String? _sourceUrl;
  String? get sourceUrl => _$this._sourceUrl;
  set sourceUrl(String? sourceUrl) => _$this._sourceUrl = sourceUrl;

  DateTime? _retrievedAt;
  DateTime? get retrievedAt => _$this._retrievedAt;
  set retrievedAt(DateTime? retrievedAt) => _$this._retrievedAt = retrievedAt;

  PriceBuilder() {
    Price._defaults(this);
  }

  PriceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _provider = $v.provider;
      _model = $v.model;
      _unit = $v.unit;
      _priceMicros = $v.priceMicros;
      _currency = $v.currency;
      _validFrom = $v.validFrom;
      _validTo = $v.validTo;
      _sourceUrl = $v.sourceUrl;
      _retrievedAt = $v.retrievedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Price other) {
    _$v = other as _$Price;
  }

  @override
  void update(void Function(PriceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Price build() => _build();

  _$Price _build() {
    final _$result =
        _$v ??
        _$Price._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Price', 'id'),
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'Price',
            'provider',
          ),
          model: BuiltValueNullFieldError.checkNotNull(
            model,
            r'Price',
            'model',
          ),
          unit: BuiltValueNullFieldError.checkNotNull(unit, r'Price', 'unit'),
          priceMicros: BuiltValueNullFieldError.checkNotNull(
            priceMicros,
            r'Price',
            'priceMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'Price',
            'currency',
          ),
          validFrom: BuiltValueNullFieldError.checkNotNull(
            validFrom,
            r'Price',
            'validFrom',
          ),
          validTo: validTo,
          sourceUrl: BuiltValueNullFieldError.checkNotNull(
            sourceUrl,
            r'Price',
            'sourceUrl',
          ),
          retrievedAt: BuiltValueNullFieldError.checkNotNull(
            retrievedAt,
            r'Price',
            'retrievedAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
