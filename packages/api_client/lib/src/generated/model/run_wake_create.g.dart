// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeCreateReasonEnum _$runWakeCreateReasonEnum_agent =
    const RunWakeCreateReasonEnum._('agent');
const RunWakeCreateReasonEnum _$runWakeCreateReasonEnum_usageLimit =
    const RunWakeCreateReasonEnum._('usageLimit');
const RunWakeCreateReasonEnum _$runWakeCreateReasonEnum_unknownDefaultOpenApi =
    const RunWakeCreateReasonEnum._('unknownDefaultOpenApi');

RunWakeCreateReasonEnum _$runWakeCreateReasonEnumValueOf(String name) {
  switch (name) {
    case 'agent':
      return _$runWakeCreateReasonEnum_agent;
    case 'usageLimit':
      return _$runWakeCreateReasonEnum_usageLimit;
    case 'unknownDefaultOpenApi':
      return _$runWakeCreateReasonEnum_unknownDefaultOpenApi;
    default:
      return _$runWakeCreateReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeCreateReasonEnum> _$runWakeCreateReasonEnumValues =
    BuiltSet<RunWakeCreateReasonEnum>(const <RunWakeCreateReasonEnum>[
      _$runWakeCreateReasonEnum_agent,
      _$runWakeCreateReasonEnum_usageLimit,
      _$runWakeCreateReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunWakeCreateReasonEnum> _$runWakeCreateReasonEnumSerializer =
    _$RunWakeCreateReasonEnumSerializer();

class _$RunWakeCreateReasonEnumSerializer
    implements PrimitiveSerializer<RunWakeCreateReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'agent': 'agent',
    'usageLimit': 'usage_limit',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'agent': 'agent',
    'usage_limit': 'usageLimit',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeCreateReasonEnum];
  @override
  final String wireName = 'RunWakeCreateReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeCreateReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeCreateReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeCreateReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunWakeCreate extends RunWakeCreate {
  @override
  final RunWakeKind kind;
  @override
  final RunWakeCreateReasonEnum reason;
  @override
  final String? label;
  @override
  final RunWakeCondition? condition;
  @override
  final DateTime? notBefore;
  @override
  final DateTime? deadline;
  @override
  final int? outage;
  @override
  final bool? deferred_;

  factory _$RunWakeCreate([void Function(RunWakeCreateBuilder)? updates]) =>
      (RunWakeCreateBuilder()..update(updates))._build();

  _$RunWakeCreate._({
    required this.kind,
    required this.reason,
    this.label,
    this.condition,
    this.notBefore,
    this.deadline,
    this.outage,
    this.deferred_,
  }) : super._();
  @override
  RunWakeCreate rebuild(void Function(RunWakeCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeCreateBuilder toBuilder() => RunWakeCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeCreate &&
        kind == other.kind &&
        reason == other.reason &&
        label == other.label &&
        condition == other.condition &&
        notBefore == other.notBefore &&
        deadline == other.deadline &&
        outage == other.outage &&
        deferred_ == other.deferred_;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, label.hashCode);
    _$hash = $jc(_$hash, condition.hashCode);
    _$hash = $jc(_$hash, notBefore.hashCode);
    _$hash = $jc(_$hash, deadline.hashCode);
    _$hash = $jc(_$hash, outage.hashCode);
    _$hash = $jc(_$hash, deferred_.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeCreate')
          ..add('kind', kind)
          ..add('reason', reason)
          ..add('label', label)
          ..add('condition', condition)
          ..add('notBefore', notBefore)
          ..add('deadline', deadline)
          ..add('outage', outage)
          ..add('deferred_', deferred_))
        .toString();
  }
}

class RunWakeCreateBuilder
    implements Builder<RunWakeCreate, RunWakeCreateBuilder> {
  _$RunWakeCreate? _$v;

  RunWakeKind? _kind;
  RunWakeKind? get kind => _$this._kind;
  set kind(RunWakeKind? kind) => _$this._kind = kind;

  RunWakeCreateReasonEnum? _reason;
  RunWakeCreateReasonEnum? get reason => _$this._reason;
  set reason(RunWakeCreateReasonEnum? reason) => _$this._reason = reason;

  String? _label;
  String? get label => _$this._label;
  set label(String? label) => _$this._label = label;

  RunWakeConditionBuilder? _condition;
  RunWakeConditionBuilder get condition =>
      _$this._condition ??= RunWakeConditionBuilder();
  set condition(RunWakeConditionBuilder? condition) =>
      _$this._condition = condition;

  DateTime? _notBefore;
  DateTime? get notBefore => _$this._notBefore;
  set notBefore(DateTime? notBefore) => _$this._notBefore = notBefore;

  DateTime? _deadline;
  DateTime? get deadline => _$this._deadline;
  set deadline(DateTime? deadline) => _$this._deadline = deadline;

  int? _outage;
  int? get outage => _$this._outage;
  set outage(int? outage) => _$this._outage = outage;

  bool? _deferred_;
  bool? get deferred_ => _$this._deferred_;
  set deferred_(bool? deferred_) => _$this._deferred_ = deferred_;

  RunWakeCreateBuilder() {
    RunWakeCreate._defaults(this);
  }

  RunWakeCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _reason = $v.reason;
      _label = $v.label;
      _condition = $v.condition?.toBuilder();
      _notBefore = $v.notBefore;
      _deadline = $v.deadline;
      _outage = $v.outage;
      _deferred_ = $v.deferred_;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeCreate other) {
    _$v = other as _$RunWakeCreate;
  }

  @override
  void update(void Function(RunWakeCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeCreate build() => _build();

  _$RunWakeCreate _build() {
    _$RunWakeCreate _$result;
    try {
      _$result =
          _$v ??
          _$RunWakeCreate._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'RunWakeCreate',
              'kind',
            ),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'RunWakeCreate',
              'reason',
            ),
            label: label,
            condition: _condition?.build(),
            notBefore: notBefore,
            deadline: deadline,
            outage: outage,
            deferred_: deferred_,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'condition';
        _condition?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunWakeCreate',
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
