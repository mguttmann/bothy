// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_source.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GroupSource _$local = const GroupSource._('local');
const GroupSource _$oidcClaim = const GroupSource._('oidcClaim');
const GroupSource _$unknownDefaultOpenApi = const GroupSource._(
  'unknownDefaultOpenApi',
);

GroupSource _$valueOf(String name) {
  switch (name) {
    case 'local':
      return _$local;
    case 'oidcClaim':
      return _$oidcClaim;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<GroupSource> _$values = BuiltSet<GroupSource>(
  const <GroupSource>[_$local, _$oidcClaim, _$unknownDefaultOpenApi],
);

class _$GroupSourceMeta {
  const _$GroupSourceMeta();
  GroupSource get local => _$local;
  GroupSource get oidcClaim => _$oidcClaim;
  GroupSource get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  GroupSource valueOf(String name) => _$valueOf(name);
  BuiltSet<GroupSource> get values => _$values;
}

mixin _$GroupSourceMixin {
  // ignore: non_constant_identifier_names
  _$GroupSourceMeta get GroupSource => const _$GroupSourceMeta();
}

Serializer<GroupSource> _$groupSourceSerializer = _$GroupSourceSerializer();

class _$GroupSourceSerializer implements PrimitiveSerializer<GroupSource> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'local': 'local',
    'oidcClaim': 'oidc_claim',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'local': 'local',
    'oidc_claim': 'oidcClaim',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GroupSource];
  @override
  final String wireName = 'GroupSource';

  @override
  Object serialize(
    Serializers serializers,
    GroupSource object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GroupSource deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GroupSource.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
