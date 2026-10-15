// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'capability.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const Capability _$approveActions = const Capability._('approveActions');
const Capability _$upload = const Capability._('upload');
const Capability _$export_ = const Capability._('export_');
const Capability _$manage = const Capability._('manage');
const Capability _$terminal = const Capability._('terminal');
const Capability _$unknownDefaultOpenApi = const Capability._(
  'unknownDefaultOpenApi',
);

Capability _$valueOf(String name) {
  switch (name) {
    case 'approveActions':
      return _$approveActions;
    case 'upload':
      return _$upload;
    case 'export_':
      return _$export_;
    case 'manage':
      return _$manage;
    case 'terminal':
      return _$terminal;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<Capability> _$values = BuiltSet<Capability>(const <Capability>[
  _$approveActions,
  _$upload,
  _$export_,
  _$manage,
  _$terminal,
  _$unknownDefaultOpenApi,
]);

class _$CapabilityMeta {
  const _$CapabilityMeta();
  Capability get approveActions => _$approveActions;
  Capability get upload => _$upload;
  Capability get export_ => _$export_;
  Capability get manage => _$manage;
  Capability get terminal => _$terminal;
  Capability get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  Capability valueOf(String name) => _$valueOf(name);
  BuiltSet<Capability> get values => _$values;
}

mixin _$CapabilityMixin {
  // ignore: non_constant_identifier_names
  _$CapabilityMeta get Capability => const _$CapabilityMeta();
}

Serializer<Capability> _$capabilitySerializer = _$CapabilitySerializer();

class _$CapabilitySerializer implements PrimitiveSerializer<Capability> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'approveActions': 'approve_actions',
    'upload': 'upload',
    'export_': 'export',
    'manage': 'manage',
    'terminal': 'terminal',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'approve_actions': 'approveActions',
    'upload': 'upload',
    'export': 'export_',
    'manage': 'manage',
    'terminal': 'terminal',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[Capability];
  @override
  final String wireName = 'Capability';

  @override
  Object serialize(
    Serializers serializers,
    Capability object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  Capability deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => Capability.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
