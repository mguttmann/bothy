// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'permission_hint_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PermissionHintKind _$h1ReadHorizon = const PermissionHintKind._(
  'h1ReadHorizon',
);
const PermissionHintKind _$grantToDisabled = const PermissionHintKind._(
  'grantToDisabled',
);
const PermissionHintKind _$triggerBlocked = const PermissionHintKind._(
  'triggerBlocked',
);
const PermissionHintKind _$agentPersonal = const PermissionHintKind._(
  'agentPersonal',
);
const PermissionHintKind _$unknownDefaultOpenApi = const PermissionHintKind._(
  'unknownDefaultOpenApi',
);

PermissionHintKind _$valueOf(String name) {
  switch (name) {
    case 'h1ReadHorizon':
      return _$h1ReadHorizon;
    case 'grantToDisabled':
      return _$grantToDisabled;
    case 'triggerBlocked':
      return _$triggerBlocked;
    case 'agentPersonal':
      return _$agentPersonal;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<PermissionHintKind> _$values = BuiltSet<PermissionHintKind>(
  const <PermissionHintKind>[
    _$h1ReadHorizon,
    _$grantToDisabled,
    _$triggerBlocked,
    _$agentPersonal,
    _$unknownDefaultOpenApi,
  ],
);

class _$PermissionHintKindMeta {
  const _$PermissionHintKindMeta();
  PermissionHintKind get h1ReadHorizon => _$h1ReadHorizon;
  PermissionHintKind get grantToDisabled => _$grantToDisabled;
  PermissionHintKind get triggerBlocked => _$triggerBlocked;
  PermissionHintKind get agentPersonal => _$agentPersonal;
  PermissionHintKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  PermissionHintKind valueOf(String name) => _$valueOf(name);
  BuiltSet<PermissionHintKind> get values => _$values;
}

mixin _$PermissionHintKindMixin {
  // ignore: non_constant_identifier_names
  _$PermissionHintKindMeta get PermissionHintKind =>
      const _$PermissionHintKindMeta();
}

Serializer<PermissionHintKind> _$permissionHintKindSerializer =
    _$PermissionHintKindSerializer();

class _$PermissionHintKindSerializer
    implements PrimitiveSerializer<PermissionHintKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'h1ReadHorizon': 'h1_read_horizon',
    'grantToDisabled': 'grant_to_disabled',
    'triggerBlocked': 'trigger_blocked',
    'agentPersonal': 'agent_personal',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'h1_read_horizon': 'h1ReadHorizon',
    'grant_to_disabled': 'grantToDisabled',
    'trigger_blocked': 'triggerBlocked',
    'agent_personal': 'agentPersonal',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PermissionHintKind];
  @override
  final String wireName = 'PermissionHintKind';

  @override
  Object serialize(
    Serializers serializers,
    PermissionHintKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PermissionHintKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PermissionHintKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
