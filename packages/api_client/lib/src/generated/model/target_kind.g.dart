// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'target_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TargetKind _$instance = const TargetKind._('instance');
const TargetKind _$space = const TargetKind._('space');
const TargetKind _$channel = const TargetKind._('channel');
const TargetKind _$host = const TargetKind._('host');
const TargetKind _$unknownDefaultOpenApi = const TargetKind._(
  'unknownDefaultOpenApi',
);

TargetKind _$valueOf(String name) {
  switch (name) {
    case 'instance':
      return _$instance;
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

final BuiltSet<TargetKind> _$values = BuiltSet<TargetKind>(const <TargetKind>[
  _$instance,
  _$space,
  _$channel,
  _$host,
  _$unknownDefaultOpenApi,
]);

class _$TargetKindMeta {
  const _$TargetKindMeta();
  TargetKind get instance => _$instance;
  TargetKind get space => _$space;
  TargetKind get channel => _$channel;
  TargetKind get host => _$host;
  TargetKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TargetKind valueOf(String name) => _$valueOf(name);
  BuiltSet<TargetKind> get values => _$values;
}

mixin _$TargetKindMixin {
  // ignore: non_constant_identifier_names
  _$TargetKindMeta get TargetKind => const _$TargetKindMeta();
}

Serializer<TargetKind> _$targetKindSerializer = _$TargetKindSerializer();

class _$TargetKindSerializer implements PrimitiveSerializer<TargetKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'instance': 'instance',
    'space': 'space',
    'channel': 'channel',
    'host': 'host',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'instance': 'instance',
    'space': 'space',
    'channel': 'channel',
    'host': 'host',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TargetKind];
  @override
  final String wireName = 'TargetKind';

  @override
  Object serialize(
    Serializers serializers,
    TargetKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TargetKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TargetKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
