// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostReport extends CostReport {
  @override
  final String provider;
  @override
  final String model;
  @override
  final int? tokensIn;
  @override
  final int? tokensOut;
  @override
  final int? tokensCacheRead;
  @override
  final int? tokensCacheWrite;
  @override
  final int? tokensCacheWrite1h;
  @override
  final int? amountMicros;
  @override
  final String currency;

  factory _$CostReport([void Function(CostReportBuilder)? updates]) =>
      (CostReportBuilder()..update(updates))._build();

  _$CostReport._({
    required this.provider,
    required this.model,
    this.tokensIn,
    this.tokensOut,
    this.tokensCacheRead,
    this.tokensCacheWrite,
    this.tokensCacheWrite1h,
    this.amountMicros,
    required this.currency,
  }) : super._();
  @override
  CostReport rebuild(void Function(CostReportBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostReportBuilder toBuilder() => CostReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostReport &&
        provider == other.provider &&
        model == other.model &&
        tokensIn == other.tokensIn &&
        tokensOut == other.tokensOut &&
        tokensCacheRead == other.tokensCacheRead &&
        tokensCacheWrite == other.tokensCacheWrite &&
        tokensCacheWrite1h == other.tokensCacheWrite1h &&
        amountMicros == other.amountMicros &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, tokensIn.hashCode);
    _$hash = $jc(_$hash, tokensOut.hashCode);
    _$hash = $jc(_$hash, tokensCacheRead.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite1h.hashCode);
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostReport')
          ..add('provider', provider)
          ..add('model', model)
          ..add('tokensIn', tokensIn)
          ..add('tokensOut', tokensOut)
          ..add('tokensCacheRead', tokensCacheRead)
          ..add('tokensCacheWrite', tokensCacheWrite)
          ..add('tokensCacheWrite1h', tokensCacheWrite1h)
          ..add('amountMicros', amountMicros)
          ..add('currency', currency))
        .toString();
  }
}

class CostReportBuilder implements Builder<CostReport, CostReportBuilder> {
  _$CostReport? _$v;

  String? _provider;
  String? get provider => _$this._provider;
  set provider(String? provider) => _$this._provider = provider;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  int? _tokensIn;
  int? get tokensIn => _$this._tokensIn;
  set tokensIn(int? tokensIn) => _$this._tokensIn = tokensIn;

  int? _tokensOut;
  int? get tokensOut => _$this._tokensOut;
  set tokensOut(int? tokensOut) => _$this._tokensOut = tokensOut;

  int? _tokensCacheRead;
  int? get tokensCacheRead => _$this._tokensCacheRead;
  set tokensCacheRead(int? tokensCacheRead) =>
      _$this._tokensCacheRead = tokensCacheRead;

  int? _tokensCacheWrite;
  int? get tokensCacheWrite => _$this._tokensCacheWrite;
  set tokensCacheWrite(int? tokensCacheWrite) =>
      _$this._tokensCacheWrite = tokensCacheWrite;

  int? _tokensCacheWrite1h;
  int? get tokensCacheWrite1h => _$this._tokensCacheWrite1h;
  set tokensCacheWrite1h(int? tokensCacheWrite1h) =>
      _$this._tokensCacheWrite1h = tokensCacheWrite1h;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  CostReportBuilder() {
    CostReport._defaults(this);
  }

  CostReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _provider = $v.provider;
      _model = $v.model;
      _tokensIn = $v.tokensIn;
      _tokensOut = $v.tokensOut;
      _tokensCacheRead = $v.tokensCacheRead;
      _tokensCacheWrite = $v.tokensCacheWrite;
      _tokensCacheWrite1h = $v.tokensCacheWrite1h;
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostReport other) {
    _$v = other as _$CostReport;
  }

  @override
  void update(void Function(CostReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostReport build() => _build();

  _$CostReport _build() {
    final _$result =
        _$v ??
        _$CostReport._(
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'CostReport',
            'provider',
          ),
          model: BuiltValueNullFieldError.checkNotNull(
            model,
            r'CostReport',
            'model',
          ),
          tokensIn: tokensIn,
          tokensOut: tokensOut,
          tokensCacheRead: tokensCacheRead,
          tokensCacheWrite: tokensCacheWrite,
          tokensCacheWrite1h: tokensCacheWrite1h,
          amountMicros: amountMicros,
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'CostReport',
            'currency',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
