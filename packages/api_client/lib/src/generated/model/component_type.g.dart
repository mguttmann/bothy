// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'component_type.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComponentType _$button = const ComponentType._('button');
const ComponentType _$select = const ComponentType._('select');
const ComponentType _$form = const ComponentType._('form');
const ComponentType _$unknownDefaultOpenApi = const ComponentType._(
  'unknownDefaultOpenApi',
);

ComponentType _$valueOf(String name) {
  switch (name) {
    case 'button':
      return _$button;
    case 'select':
      return _$select;
    case 'form':
      return _$form;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ComponentType> _$values = BuiltSet<ComponentType>(
  const <ComponentType>[_$button, _$select, _$form, _$unknownDefaultOpenApi],
);

class _$ComponentTypeMeta {
  const _$ComponentTypeMeta();
  ComponentType get button => _$button;
  ComponentType get select => _$select;
  ComponentType get form => _$form;
  ComponentType get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ComponentType valueOf(String name) => _$valueOf(name);
  BuiltSet<ComponentType> get values => _$values;
}

mixin _$ComponentTypeMixin {
  // ignore: non_constant_identifier_names
  _$ComponentTypeMeta get ComponentType => const _$ComponentTypeMeta();
}

Serializer<ComponentType> _$componentTypeSerializer =
    _$ComponentTypeSerializer();

class _$ComponentTypeSerializer implements PrimitiveSerializer<ComponentType> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'button': 'button',
    'select': 'select',
    'form': 'form',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'button': 'button',
    'select': 'select',
    'form': 'form',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComponentType];
  @override
  final String wireName = 'ComponentType';

  @override
  Object serialize(
    Serializers serializers,
    ComponentType object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComponentType deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComponentType.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
