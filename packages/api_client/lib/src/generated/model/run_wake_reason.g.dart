// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_reason.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeReason _$agent = const RunWakeReason._('agent');
const RunWakeReason _$recovery = const RunWakeReason._('recovery');
const RunWakeReason _$usageLimit = const RunWakeReason._('usageLimit');
const RunWakeReason _$unknownDefaultOpenApi = const RunWakeReason._(
  'unknownDefaultOpenApi',
);

RunWakeReason _$valueOf(String name) {
  switch (name) {
    case 'agent':
      return _$agent;
    case 'recovery':
      return _$recovery;
    case 'usageLimit':
      return _$usageLimit;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeReason> _$values = BuiltSet<RunWakeReason>(
  const <RunWakeReason>[
    _$agent,
    _$recovery,
    _$usageLimit,
    _$unknownDefaultOpenApi,
  ],
);

class _$RunWakeReasonMeta {
  const _$RunWakeReasonMeta();
  RunWakeReason get agent => _$agent;
  RunWakeReason get recovery => _$recovery;
  RunWakeReason get usageLimit => _$usageLimit;
  RunWakeReason get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  RunWakeReason valueOf(String name) => _$valueOf(name);
  BuiltSet<RunWakeReason> get values => _$values;
}

mixin _$RunWakeReasonMixin {
  // ignore: non_constant_identifier_names
  _$RunWakeReasonMeta get RunWakeReason => const _$RunWakeReasonMeta();
}

Serializer<RunWakeReason> _$runWakeReasonSerializer =
    _$RunWakeReasonSerializer();

class _$RunWakeReasonSerializer implements PrimitiveSerializer<RunWakeReason> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'agent': 'agent',
    'recovery': 'recovery',
    'usageLimit': 'usage_limit',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'agent': 'agent',
    'recovery': 'recovery',
    'usage_limit': 'usageLimit',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeReason];
  @override
  final String wireName = 'RunWakeReason';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeReason object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeReason deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeReason.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
