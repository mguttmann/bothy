// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_pause_reason.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TeamPauseReason _$turnBudget = const TeamPauseReason._('turnBudget');
const TeamPauseReason _$pingpong = const TeamPauseReason._('pingpong');
const TeamPauseReason _$stopped = const TeamPauseReason._('stopped');
const TeamPauseReason _$unknownDefaultOpenApi = const TeamPauseReason._(
  'unknownDefaultOpenApi',
);

TeamPauseReason _$valueOf(String name) {
  switch (name) {
    case 'turnBudget':
      return _$turnBudget;
    case 'pingpong':
      return _$pingpong;
    case 'stopped':
      return _$stopped;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<TeamPauseReason> _$values = BuiltSet<TeamPauseReason>(
  const <TeamPauseReason>[
    _$turnBudget,
    _$pingpong,
    _$stopped,
    _$unknownDefaultOpenApi,
  ],
);

class _$TeamPauseReasonMeta {
  const _$TeamPauseReasonMeta();
  TeamPauseReason get turnBudget => _$turnBudget;
  TeamPauseReason get pingpong => _$pingpong;
  TeamPauseReason get stopped => _$stopped;
  TeamPauseReason get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TeamPauseReason valueOf(String name) => _$valueOf(name);
  BuiltSet<TeamPauseReason> get values => _$values;
}

mixin _$TeamPauseReasonMixin {
  // ignore: non_constant_identifier_names
  _$TeamPauseReasonMeta get TeamPauseReason => const _$TeamPauseReasonMeta();
}

Serializer<TeamPauseReason> _$teamPauseReasonSerializer =
    _$TeamPauseReasonSerializer();

class _$TeamPauseReasonSerializer
    implements PrimitiveSerializer<TeamPauseReason> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'turnBudget': 'turn_budget',
    'pingpong': 'pingpong',
    'stopped': 'stopped',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'turn_budget': 'turnBudget',
    'pingpong': 'pingpong',
    'stopped': 'stopped',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TeamPauseReason];
  @override
  final String wireName = 'TeamPauseReason';

  @override
  Object serialize(
    Serializers serializers,
    TeamPauseReason object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TeamPauseReason deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TeamPauseReason.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
