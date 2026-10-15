// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_runtime_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentRuntimeState _$unknown = const AgentRuntimeState._('unknown');
const AgentRuntimeState _$online = const AgentRuntimeState._('online');
const AgentRuntimeState _$degraded = const AgentRuntimeState._('degraded');
const AgentRuntimeState _$offline = const AgentRuntimeState._('offline');
const AgentRuntimeState _$unknownDefaultOpenApi = const AgentRuntimeState._(
  'unknownDefaultOpenApi',
);

AgentRuntimeState _$valueOf(String name) {
  switch (name) {
    case 'unknown':
      return _$unknown;
    case 'online':
      return _$online;
    case 'degraded':
      return _$degraded;
    case 'offline':
      return _$offline;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentRuntimeState> _$values = BuiltSet<AgentRuntimeState>(
  const <AgentRuntimeState>[
    _$unknown,
    _$online,
    _$degraded,
    _$offline,
    _$unknownDefaultOpenApi,
  ],
);

class _$AgentRuntimeStateMeta {
  const _$AgentRuntimeStateMeta();
  AgentRuntimeState get unknown => _$unknown;
  AgentRuntimeState get online => _$online;
  AgentRuntimeState get degraded => _$degraded;
  AgentRuntimeState get offline => _$offline;
  AgentRuntimeState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  AgentRuntimeState valueOf(String name) => _$valueOf(name);
  BuiltSet<AgentRuntimeState> get values => _$values;
}

mixin _$AgentRuntimeStateMixin {
  // ignore: non_constant_identifier_names
  _$AgentRuntimeStateMeta get AgentRuntimeState =>
      const _$AgentRuntimeStateMeta();
}

Serializer<AgentRuntimeState> _$agentRuntimeStateSerializer =
    _$AgentRuntimeStateSerializer();

class _$AgentRuntimeStateSerializer
    implements PrimitiveSerializer<AgentRuntimeState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'unknown': 'unknown',
    'online': 'online',
    'degraded': 'degraded',
    'offline': 'offline',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'unknown': 'unknown',
    'online': 'online',
    'degraded': 'degraded',
    'offline': 'offline',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentRuntimeState];
  @override
  final String wireName = 'AgentRuntimeState';

  @override
  Object serialize(
    Serializers serializers,
    AgentRuntimeState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentRuntimeState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentRuntimeState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
