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
  final Money cost;

  factory _$RunCost([void Function(RunCostBuilder)? updates]) =>
      (RunCostBuilder()..update(updates))._build();

  _$RunCost._({
    required this.runId,
    required this.billingMode,
    required this.cost,
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
        cost == other.cost;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, cost.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunCost')
          ..add('runId', runId)
          ..add('billingMode', billingMode)
          ..add('cost', cost))
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

  MoneyBuilder? _cost;
  MoneyBuilder get cost => _$this._cost ??= MoneyBuilder();
  set cost(MoneyBuilder? cost) => _$this._cost = cost;

  RunCostBuilder() {
    RunCost._defaults(this);
  }

  RunCostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _billingMode = $v.billingMode;
      _cost = $v.cost.toBuilder();
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
            cost: cost.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cost';
        cost.build();
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
