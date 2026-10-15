// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_step.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsLiveStepKindEnum _$statsLiveStepKindEnum_toolCall =
    const StatsLiveStepKindEnum._('toolCall');
const StatsLiveStepKindEnum _$statsLiveStepKindEnum_status =
    const StatsLiveStepKindEnum._('status');
const StatsLiveStepKindEnum _$statsLiveStepKindEnum_subagent =
    const StatsLiveStepKindEnum._('subagent');
const StatsLiveStepKindEnum _$statsLiveStepKindEnum_workflow =
    const StatsLiveStepKindEnum._('workflow');
const StatsLiveStepKindEnum _$statsLiveStepKindEnum_backgroundJob =
    const StatsLiveStepKindEnum._('backgroundJob');
const StatsLiveStepKindEnum _$statsLiveStepKindEnum_unknownDefaultOpenApi =
    const StatsLiveStepKindEnum._('unknownDefaultOpenApi');

StatsLiveStepKindEnum _$statsLiveStepKindEnumValueOf(String name) {
  switch (name) {
    case 'toolCall':
      return _$statsLiveStepKindEnum_toolCall;
    case 'status':
      return _$statsLiveStepKindEnum_status;
    case 'subagent':
      return _$statsLiveStepKindEnum_subagent;
    case 'workflow':
      return _$statsLiveStepKindEnum_workflow;
    case 'backgroundJob':
      return _$statsLiveStepKindEnum_backgroundJob;
    case 'unknownDefaultOpenApi':
      return _$statsLiveStepKindEnum_unknownDefaultOpenApi;
    default:
      return _$statsLiveStepKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsLiveStepKindEnum> _$statsLiveStepKindEnumValues =
    BuiltSet<StatsLiveStepKindEnum>(const <StatsLiveStepKindEnum>[
      _$statsLiveStepKindEnum_toolCall,
      _$statsLiveStepKindEnum_status,
      _$statsLiveStepKindEnum_subagent,
      _$statsLiveStepKindEnum_workflow,
      _$statsLiveStepKindEnum_backgroundJob,
      _$statsLiveStepKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<StatsLiveStepKindEnum> _$statsLiveStepKindEnumSerializer =
    _$StatsLiveStepKindEnumSerializer();

class _$StatsLiveStepKindEnumSerializer
    implements PrimitiveSerializer<StatsLiveStepKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'toolCall': 'tool_call',
    'status': 'status',
    'subagent': 'subagent',
    'workflow': 'workflow',
    'backgroundJob': 'background_job',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'tool_call': 'toolCall',
    'status': 'status',
    'subagent': 'subagent',
    'workflow': 'workflow',
    'background_job': 'backgroundJob',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsLiveStepKindEnum];
  @override
  final String wireName = 'StatsLiveStepKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveStepKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsLiveStepKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsLiveStepKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsLiveStep extends StatsLiveStep {
  @override
  final StatsLiveStepKindEnum kind;
  @override
  final String name;

  factory _$StatsLiveStep([void Function(StatsLiveStepBuilder)? updates]) =>
      (StatsLiveStepBuilder()..update(updates))._build();

  _$StatsLiveStep._({required this.kind, required this.name}) : super._();
  @override
  StatsLiveStep rebuild(void Function(StatsLiveStepBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveStepBuilder toBuilder() => StatsLiveStepBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveStep && kind == other.kind && name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveStep')
          ..add('kind', kind)
          ..add('name', name))
        .toString();
  }
}

class StatsLiveStepBuilder
    implements Builder<StatsLiveStep, StatsLiveStepBuilder> {
  _$StatsLiveStep? _$v;

  StatsLiveStepKindEnum? _kind;
  StatsLiveStepKindEnum? get kind => _$this._kind;
  set kind(StatsLiveStepKindEnum? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  StatsLiveStepBuilder() {
    StatsLiveStep._defaults(this);
  }

  StatsLiveStepBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveStep other) {
    _$v = other as _$StatsLiveStep;
  }

  @override
  void update(void Function(StatsLiveStepBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveStep build() => _build();

  _$StatsLiveStep _build() {
    final _$result =
        _$v ??
        _$StatsLiveStep._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'StatsLiveStep',
            'kind',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'StatsLiveStep',
            'name',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
