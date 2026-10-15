// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_platform.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostPlatform _$macos = const HostPlatform._('macos');
const HostPlatform _$linux = const HostPlatform._('linux');
const HostPlatform _$windows = const HostPlatform._('windows');
const HostPlatform _$unknownDefaultOpenApi = const HostPlatform._(
  'unknownDefaultOpenApi',
);

HostPlatform _$valueOf(String name) {
  switch (name) {
    case 'macos':
      return _$macos;
    case 'linux':
      return _$linux;
    case 'windows':
      return _$windows;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<HostPlatform> _$values = BuiltSet<HostPlatform>(
  const <HostPlatform>[_$macos, _$linux, _$windows, _$unknownDefaultOpenApi],
);

class _$HostPlatformMeta {
  const _$HostPlatformMeta();
  HostPlatform get macos => _$macos;
  HostPlatform get linux => _$linux;
  HostPlatform get windows => _$windows;
  HostPlatform get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  HostPlatform valueOf(String name) => _$valueOf(name);
  BuiltSet<HostPlatform> get values => _$values;
}

mixin _$HostPlatformMixin {
  // ignore: non_constant_identifier_names
  _$HostPlatformMeta get HostPlatform => const _$HostPlatformMeta();
}

Serializer<HostPlatform> _$hostPlatformSerializer = _$HostPlatformSerializer();

class _$HostPlatformSerializer implements PrimitiveSerializer<HostPlatform> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'macos': 'macos',
    'linux': 'linux',
    'windows': 'windows',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'macos': 'macos',
    'linux': 'linux',
    'windows': 'windows',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostPlatform];
  @override
  final String wireName = 'HostPlatform';

  @override
  Object serialize(
    Serializers serializers,
    HostPlatform object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostPlatform deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostPlatform.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
