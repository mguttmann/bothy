// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'price_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PriceCreate extends PriceCreate {
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

  factory _$PriceCreate([void Function(PriceCreateBuilder)? updates]) =>
      (PriceCreateBuilder()..update(updates))._build();

  _$PriceCreate._({
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
  PriceCreate rebuild(void Function(PriceCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PriceCreateBuilder toBuilder() => PriceCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PriceCreate &&
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
    return (newBuiltValueToStringHelper(r'PriceCreate')
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

class PriceCreateBuilder implements Builder<PriceCreate, PriceCreateBuilder> {
  _$PriceCreate? _$v;

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

  PriceCreateBuilder() {
    PriceCreate._defaults(this);
  }

  PriceCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
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
  void replace(PriceCreate other) {
    _$v = other as _$PriceCreate;
  }

  @override
  void update(void Function(PriceCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PriceCreate build() => _build();

  _$PriceCreate _build() {
    final _$result =
        _$v ??
        _$PriceCreate._(
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'PriceCreate',
            'provider',
          ),
          model: BuiltValueNullFieldError.checkNotNull(
            model,
            r'PriceCreate',
            'model',
          ),
          unit: BuiltValueNullFieldError.checkNotNull(
            unit,
            r'PriceCreate',
            'unit',
          ),
          priceMicros: BuiltValueNullFieldError.checkNotNull(
            priceMicros,
            r'PriceCreate',
            'priceMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'PriceCreate',
            'currency',
          ),
          validFrom: BuiltValueNullFieldError.checkNotNull(
            validFrom,
            r'PriceCreate',
            'validFrom',
          ),
          validTo: validTo,
          sourceUrl: BuiltValueNullFieldError.checkNotNull(
            sourceUrl,
            r'PriceCreate',
            'sourceUrl',
          ),
          retrievedAt: BuiltValueNullFieldError.checkNotNull(
            retrievedAt,
            r'PriceCreate',
            'retrievedAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
