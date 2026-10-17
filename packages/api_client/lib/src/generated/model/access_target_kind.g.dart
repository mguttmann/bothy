// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_target_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AccessTargetKind _$space = const AccessTargetKind._('space');
const AccessTargetKind _$channel = const AccessTargetKind._('channel');
const AccessTargetKind _$host = const AccessTargetKind._('host');
const AccessTargetKind _$unknownDefaultOpenApi = const AccessTargetKind._(
  'unknownDefaultOpenApi',
);

AccessTargetKind _$valueOf(String name) {
  switch (name) {
    case 'space':
      return _$space;
    case 'channel':
      return _$channel;
    case 'host':
      return _$host;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<AccessTargetKind> _$values = BuiltSet<AccessTargetKind>(
  const <AccessTargetKind>[_$space, _$channel, _$host, _$unknownDefaultOpenApi],
);

class _$AccessTargetKindMeta {
  const _$AccessTargetKindMeta();
  AccessTargetKind get space => _$space;
  AccessTargetKind get channel => _$channel;
  AccessTargetKind get host => _$host;
  AccessTargetKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  AccessTargetKind valueOf(String name) => _$valueOf(name);
  BuiltSet<AccessTargetKind> get values => _$values;
}

mixin _$AccessTargetKindMixin {
  // ignore: non_constant_identifier_names
  _$AccessTargetKindMeta get AccessTargetKind => const _$AccessTargetKindMeta();
}

Serializer<AccessTargetKind> _$accessTargetKindSerializer =
    _$AccessTargetKindSerializer();

class _$AccessTargetKindSerializer
    implements PrimitiveSerializer<AccessTargetKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'space': 'space',
    'channel': 'channel',
    'host': 'host',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'space': 'space',
    'channel': 'channel',
    'host': 'host',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AccessTargetKind];
  @override
  final String wireName = 'AccessTargetKind';

  @override
  Object serialize(
    Serializers serializers,
    AccessTargetKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AccessTargetKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AccessTargetKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
