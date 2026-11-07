// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_fire.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeFireOutcomeEnum _$runWakeFireOutcomeEnum_fired =
    const RunWakeFireOutcomeEnum._('fired');
const RunWakeFireOutcomeEnum _$runWakeFireOutcomeEnum_failed =
    const RunWakeFireOutcomeEnum._('failed');
const RunWakeFireOutcomeEnum _$runWakeFireOutcomeEnum_inputSkipped =
    const RunWakeFireOutcomeEnum._('inputSkipped');
const RunWakeFireOutcomeEnum _$runWakeFireOutcomeEnum_unknownDefaultOpenApi =
    const RunWakeFireOutcomeEnum._('unknownDefaultOpenApi');

RunWakeFireOutcomeEnum _$runWakeFireOutcomeEnumValueOf(String name) {
  switch (name) {
    case 'fired':
      return _$runWakeFireOutcomeEnum_fired;
    case 'failed':
      return _$runWakeFireOutcomeEnum_failed;
    case 'inputSkipped':
      return _$runWakeFireOutcomeEnum_inputSkipped;
    case 'unknownDefaultOpenApi':
      return _$runWakeFireOutcomeEnum_unknownDefaultOpenApi;
    default:
      return _$runWakeFireOutcomeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeFireOutcomeEnum> _$runWakeFireOutcomeEnumValues =
    BuiltSet<RunWakeFireOutcomeEnum>(const <RunWakeFireOutcomeEnum>[
      _$runWakeFireOutcomeEnum_fired,
      _$runWakeFireOutcomeEnum_failed,
      _$runWakeFireOutcomeEnum_inputSkipped,
      _$runWakeFireOutcomeEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunWakeFireOutcomeEnum> _$runWakeFireOutcomeEnumSerializer =
    _$RunWakeFireOutcomeEnumSerializer();

class _$RunWakeFireOutcomeEnumSerializer
    implements PrimitiveSerializer<RunWakeFireOutcomeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'fired': 'fired',
    'failed': 'failed',
    'inputSkipped': 'input_skipped',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'fired': 'fired',
    'failed': 'failed',
    'input_skipped': 'inputSkipped',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeFireOutcomeEnum];
  @override
  final String wireName = 'RunWakeFireOutcomeEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeFireOutcomeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeFireOutcomeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeFireOutcomeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunWakeFire extends RunWakeFire {
  @override
  final RunWakeFireOutcomeEnum outcome;
  @override
  final String? resultText;
  @override
  final String? reason;
  @override
  final String? runId;

  factory _$RunWakeFire([void Function(RunWakeFireBuilder)? updates]) =>
      (RunWakeFireBuilder()..update(updates))._build();

  _$RunWakeFire._({
    required this.outcome,
    this.resultText,
    this.reason,
    this.runId,
  }) : super._();
  @override
  RunWakeFire rebuild(void Function(RunWakeFireBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeFireBuilder toBuilder() => RunWakeFireBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeFire &&
        outcome == other.outcome &&
        resultText == other.resultText &&
        reason == other.reason &&
        runId == other.runId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, outcome.hashCode);
    _$hash = $jc(_$hash, resultText.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeFire')
          ..add('outcome', outcome)
          ..add('resultText', resultText)
          ..add('reason', reason)
          ..add('runId', runId))
        .toString();
  }
}

class RunWakeFireBuilder implements Builder<RunWakeFire, RunWakeFireBuilder> {
  _$RunWakeFire? _$v;

  RunWakeFireOutcomeEnum? _outcome;
  RunWakeFireOutcomeEnum? get outcome => _$this._outcome;
  set outcome(RunWakeFireOutcomeEnum? outcome) => _$this._outcome = outcome;

  String? _resultText;
  String? get resultText => _$this._resultText;
  set resultText(String? resultText) => _$this._resultText = resultText;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  RunWakeFireBuilder() {
    RunWakeFire._defaults(this);
  }

  RunWakeFireBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _outcome = $v.outcome;
      _resultText = $v.resultText;
      _reason = $v.reason;
      _runId = $v.runId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeFire other) {
    _$v = other as _$RunWakeFire;
  }

  @override
  void update(void Function(RunWakeFireBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeFire build() => _build();

  _$RunWakeFire _build() {
    final _$result =
        _$v ??
        _$RunWakeFire._(
          outcome: BuiltValueNullFieldError.checkNotNull(
            outcome,
            r'RunWakeFire',
            'outcome',
          ),
          resultText: resultText,
          reason: reason,
          runId: runId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
