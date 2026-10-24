// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_cost.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunCost extends RunCost {
  @override
  final String runId;
  @override
  final BillingMode billingMode;
  @override
  final String? agentSessionId;
  @override
  final TokenCounts tokens;
  @override
  final BuiltList<Money>? costs;
  @override
  final bool unpriced;

  factory _$RunCost([void Function(RunCostBuilder)? updates]) =>
      (RunCostBuilder()..update(updates))._build();

  _$RunCost._({
    required this.runId,
    required this.billingMode,
    this.agentSessionId,
    required this.tokens,
    this.costs,
    required this.unpriced,
  }) : super._();
  @override
  RunCost rebuild(void Function(RunCostBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunCostBuilder toBuilder() => RunCostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunCost &&
        runId == other.runId &&
        billingMode == other.billingMode &&
        agentSessionId == other.agentSessionId &&
        tokens == other.tokens &&
        costs == other.costs &&
        unpriced == other.unpriced;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, agentSessionId.hashCode);
    _$hash = $jc(_$hash, tokens.hashCode);
    _$hash = $jc(_$hash, costs.hashCode);
    _$hash = $jc(_$hash, unpriced.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunCost')
          ..add('runId', runId)
          ..add('billingMode', billingMode)
          ..add('agentSessionId', agentSessionId)
          ..add('tokens', tokens)
          ..add('costs', costs)
          ..add('unpriced', unpriced))
        .toString();
  }
}

class RunCostBuilder implements Builder<RunCost, RunCostBuilder> {
  _$RunCost? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  String? _agentSessionId;
  String? get agentSessionId => _$this._agentSessionId;
  set agentSessionId(String? agentSessionId) =>
      _$this._agentSessionId = agentSessionId;

  TokenCountsBuilder? _tokens;
  TokenCountsBuilder get tokens => _$this._tokens ??= TokenCountsBuilder();
  set tokens(TokenCountsBuilder? tokens) => _$this._tokens = tokens;

  ListBuilder<Money>? _costs;
  ListBuilder<Money> get costs => _$this._costs ??= ListBuilder<Money>();
  set costs(ListBuilder<Money>? costs) => _$this._costs = costs;

  bool? _unpriced;
  bool? get unpriced => _$this._unpriced;
  set unpriced(bool? unpriced) => _$this._unpriced = unpriced;

  RunCostBuilder() {
    RunCost._defaults(this);
  }

  RunCostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _billingMode = $v.billingMode;
      _agentSessionId = $v.agentSessionId;
      _tokens = $v.tokens.toBuilder();
      _costs = $v.costs?.toBuilder();
      _unpriced = $v.unpriced;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunCost other) {
    _$v = other as _$RunCost;
  }

  @override
  void update(void Function(RunCostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunCost build() => _build();

  _$RunCost _build() {
    _$RunCost _$result;
    try {
      _$result =
          _$v ??
          _$RunCost._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'RunCost',
              'runId',
            ),
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'RunCost',
              'billingMode',
            ),
            agentSessionId: agentSessionId,
            tokens: tokens.build(),
            costs: _costs?.build(),
            unpriced: BuiltValueNullFieldError.checkNotNull(
              unpriced,
              r'RunCost',
              'unpriced',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'tokens';
        tokens.build();
        _$failedField = 'costs';
        _costs?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunCost',
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
