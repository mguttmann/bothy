// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'operator_mode.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const OperatorMode _$inherit = const OperatorMode._('inherit');
const OperatorMode _$none = const OperatorMode._('none');
const OperatorMode _$agent = const OperatorMode._('agent');
const OperatorMode _$unknownDefaultOpenApi = const OperatorMode._(
  'unknownDefaultOpenApi',
);

OperatorMode _$valueOf(String name) {
  switch (name) {
    case 'inherit':
      return _$inherit;
    case 'none':
      return _$none;
    case 'agent':
      return _$agent;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<OperatorMode> _$values = BuiltSet<OperatorMode>(
  const <OperatorMode>[_$inherit, _$none, _$agent, _$unknownDefaultOpenApi],
);

class _$OperatorModeMeta {
  const _$OperatorModeMeta();
  OperatorMode get inherit => _$inherit;
  OperatorMode get none => _$none;
  OperatorMode get agent => _$agent;
  OperatorMode get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  OperatorMode valueOf(String name) => _$valueOf(name);
  BuiltSet<OperatorMode> get values => _$values;
}

mixin _$OperatorModeMixin {
  // ignore: non_constant_identifier_names
  _$OperatorModeMeta get OperatorMode => const _$OperatorModeMeta();
}

Serializer<OperatorMode> _$operatorModeSerializer = _$OperatorModeSerializer();

class _$OperatorModeSerializer implements PrimitiveSerializer<OperatorMode> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'inherit': 'inherit',
    'none': 'none',
    'agent': 'agent',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'inherit': 'inherit',
    'none': 'none',
    'agent': 'agent',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[OperatorMode];
  @override
  final String wireName = 'OperatorMode';

  @override
  Object serialize(
    Serializers serializers,
    OperatorMode object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  OperatorMode deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => OperatorMode.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
