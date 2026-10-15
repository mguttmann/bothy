// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_finish.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunFinishStateEnum _$runFinishStateEnum_completed =
    const RunFinishStateEnum._('completed');
const RunFinishStateEnum _$runFinishStateEnum_failed =
    const RunFinishStateEnum._('failed');
const RunFinishStateEnum _$runFinishStateEnum_cancelled =
    const RunFinishStateEnum._('cancelled');
const RunFinishStateEnum _$runFinishStateEnum_unknownDefaultOpenApi =
    const RunFinishStateEnum._('unknownDefaultOpenApi');

RunFinishStateEnum _$runFinishStateEnumValueOf(String name) {
  switch (name) {
    case 'completed':
      return _$runFinishStateEnum_completed;
    case 'failed':
      return _$runFinishStateEnum_failed;
    case 'cancelled':
      return _$runFinishStateEnum_cancelled;
    case 'unknownDefaultOpenApi':
      return _$runFinishStateEnum_unknownDefaultOpenApi;
    default:
      return _$runFinishStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunFinishStateEnum> _$runFinishStateEnumValues =
    BuiltSet<RunFinishStateEnum>(const <RunFinishStateEnum>[
      _$runFinishStateEnum_completed,
      _$runFinishStateEnum_failed,
      _$runFinishStateEnum_cancelled,
      _$runFinishStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunFinishStateEnum> _$runFinishStateEnumSerializer =
    _$RunFinishStateEnumSerializer();

class _$RunFinishStateEnumSerializer
    implements PrimitiveSerializer<RunFinishStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunFinishStateEnum];
  @override
  final String wireName = 'RunFinishStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunFinishStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunFinishStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunFinishStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunFinish extends RunFinish {
  @override
  final RunFinishStateEnum state;
  @override
  final String? reason;
  @override
  final String? summary;
  @override
  final RunMetrics? metrics;
  @override
  final BuiltList<CostReport>? costs;

  factory _$RunFinish([void Function(RunFinishBuilder)? updates]) =>
      (RunFinishBuilder()..update(updates))._build();

  _$RunFinish._({
    required this.state,
    this.reason,
    this.summary,
    this.metrics,
    this.costs,
  }) : super._();
  @override
  RunFinish rebuild(void Function(RunFinishBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunFinishBuilder toBuilder() => RunFinishBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunFinish &&
        state == other.state &&
        reason == other.reason &&
        summary == other.summary &&
        metrics == other.metrics &&
        costs == other.costs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, summary.hashCode);
    _$hash = $jc(_$hash, metrics.hashCode);
    _$hash = $jc(_$hash, costs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunFinish')
          ..add('state', state)
          ..add('reason', reason)
          ..add('summary', summary)
          ..add('metrics', metrics)
          ..add('costs', costs))
        .toString();
  }
}

class RunFinishBuilder implements Builder<RunFinish, RunFinishBuilder> {
  _$RunFinish? _$v;

  RunFinishStateEnum? _state;
  RunFinishStateEnum? get state => _$this._state;
  set state(RunFinishStateEnum? state) => _$this._state = state;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  String? _summary;
  String? get summary => _$this._summary;
  set summary(String? summary) => _$this._summary = summary;

  RunMetricsBuilder? _metrics;
  RunMetricsBuilder get metrics => _$this._metrics ??= RunMetricsBuilder();
  set metrics(RunMetricsBuilder? metrics) => _$this._metrics = metrics;

  ListBuilder<CostReport>? _costs;
  ListBuilder<CostReport> get costs =>
      _$this._costs ??= ListBuilder<CostReport>();
  set costs(ListBuilder<CostReport>? costs) => _$this._costs = costs;

  RunFinishBuilder() {
    RunFinish._defaults(this);
  }

  RunFinishBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _reason = $v.reason;
      _summary = $v.summary;
      _metrics = $v.metrics?.toBuilder();
      _costs = $v.costs?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunFinish other) {
    _$v = other as _$RunFinish;
  }

  @override
  void update(void Function(RunFinishBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunFinish build() => _build();

  _$RunFinish _build() {
    _$RunFinish _$result;
    try {
      _$result =
          _$v ??
          _$RunFinish._(
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'RunFinish',
              'state',
            ),
            reason: reason,
            summary: summary,
            metrics: _metrics?.build(),
            costs: _costs?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'metrics';
        _metrics?.build();
        _$failedField = 'costs';
        _costs?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunFinish',
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
