// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_provider_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const IdentityProviderKind _$oidc = const IdentityProviderKind._('oidc');
const IdentityProviderKind _$trustedHeader = const IdentityProviderKind._(
  'trustedHeader',
);
const IdentityProviderKind _$local = const IdentityProviderKind._('local');
const IdentityProviderKind _$unknownDefaultOpenApi =
    const IdentityProviderKind._('unknownDefaultOpenApi');

IdentityProviderKind _$valueOf(String name) {
  switch (name) {
    case 'oidc':
      return _$oidc;
    case 'trustedHeader':
      return _$trustedHeader;
    case 'local':
      return _$local;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<IdentityProviderKind> _$values = BuiltSet<IdentityProviderKind>(
  const <IdentityProviderKind>[
    _$oidc,
    _$trustedHeader,
    _$local,
    _$unknownDefaultOpenApi,
  ],
);

class _$IdentityProviderKindMeta {
  const _$IdentityProviderKindMeta();
  IdentityProviderKind get oidc => _$oidc;
  IdentityProviderKind get trustedHeader => _$trustedHeader;
  IdentityProviderKind get local => _$local;
  IdentityProviderKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  IdentityProviderKind valueOf(String name) => _$valueOf(name);
  BuiltSet<IdentityProviderKind> get values => _$values;
}

mixin _$IdentityProviderKindMixin {
  // ignore: non_constant_identifier_names
  _$IdentityProviderKindMeta get IdentityProviderKind =>
      const _$IdentityProviderKindMeta();
}

Serializer<IdentityProviderKind> _$identityProviderKindSerializer =
    _$IdentityProviderKindSerializer();

class _$IdentityProviderKindSerializer
    implements PrimitiveSerializer<IdentityProviderKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'oidc': 'oidc',
    'trustedHeader': 'trusted_header',
    'local': 'local',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'oidc': 'oidc',
    'trusted_header': 'trustedHeader',
    'local': 'local',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[IdentityProviderKind];
  @override
  final String wireName = 'IdentityProviderKind';

  @override
  Object serialize(
    Serializers serializers,
    IdentityProviderKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  IdentityProviderKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => IdentityProviderKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
