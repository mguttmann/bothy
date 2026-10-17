// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_estimate.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostEstimate extends CostEstimate {
  @override
  final String runId;
  @override
  final String agentId;
  @override
  final String provider;
  @override
  final String currency;
  @override
  final int estimateMicros;
  @override
  final bool reconciled;

  factory _$CostEstimate([void Function(CostEstimateBuilder)? updates]) =>
      (CostEstimateBuilder()..update(updates))._build();

  _$CostEstimate._({
    required this.runId,
    required this.agentId,
    required this.provider,
    required this.currency,
    required this.estimateMicros,
    required this.reconciled,
  }) : super._();
  @override
  CostEstimate rebuild(void Function(CostEstimateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostEstimateBuilder toBuilder() => CostEstimateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostEstimate &&
        runId == other.runId &&
        agentId == other.agentId &&
        provider == other.provider &&
        currency == other.currency &&
        estimateMicros == other.estimateMicros &&
        reconciled == other.reconciled;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, estimateMicros.hashCode);
    _$hash = $jc(_$hash, reconciled.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostEstimate')
          ..add('runId', runId)
          ..add('agentId', agentId)
          ..add('provider', provider)
          ..add('currency', currency)
          ..add('estimateMicros', estimateMicros)
          ..add('reconciled', reconciled))
        .toString();
  }
}

class CostEstimateBuilder
    implements Builder<CostEstimate, CostEstimateBuilder> {
  _$CostEstimate? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _provider;
  String? get provider => _$this._provider;
  set provider(String? provider) => _$this._provider = provider;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  int? _estimateMicros;
  int? get estimateMicros => _$this._estimateMicros;
  set estimateMicros(int? estimateMicros) =>
      _$this._estimateMicros = estimateMicros;

  bool? _reconciled;
  bool? get reconciled => _$this._reconciled;
  set reconciled(bool? reconciled) => _$this._reconciled = reconciled;

  CostEstimateBuilder() {
    CostEstimate._defaults(this);
  }

  CostEstimateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _agentId = $v.agentId;
      _provider = $v.provider;
      _currency = $v.currency;
      _estimateMicros = $v.estimateMicros;
      _reconciled = $v.reconciled;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostEstimate other) {
    _$v = other as _$CostEstimate;
  }

  @override
  void update(void Function(CostEstimateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostEstimate build() => _build();

  _$CostEstimate _build() {
    final _$result =
        _$v ??
        _$CostEstimate._(
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'CostEstimate',
            'runId',
          ),
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'CostEstimate',
            'agentId',
          ),
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'CostEstimate',
            'provider',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'CostEstimate',
            'currency',
          ),
          estimateMicros: BuiltValueNullFieldError.checkNotNull(
            estimateMicros,
            r'CostEstimate',
            'estimateMicros',
          ),
          reconciled: BuiltValueNullFieldError.checkNotNull(
            reconciled,
            r'CostEstimate',
            'reconciled',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
