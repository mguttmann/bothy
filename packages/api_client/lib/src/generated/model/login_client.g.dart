// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'login_client.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const LoginClient _$app = const LoginClient._('app');
const LoginClient _$console = const LoginClient._('console');
const LoginClient _$unknownDefaultOpenApi = const LoginClient._(
  'unknownDefaultOpenApi',
);

LoginClient _$valueOf(String name) {
  switch (name) {
    case 'app':
      return _$app;
    case 'console':
      return _$console;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<LoginClient> _$values = BuiltSet<LoginClient>(
  const <LoginClient>[_$app, _$console, _$unknownDefaultOpenApi],
);

class _$LoginClientMeta {
  const _$LoginClientMeta();
  LoginClient get app => _$app;
  LoginClient get console => _$console;
  LoginClient get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  LoginClient valueOf(String name) => _$valueOf(name);
  BuiltSet<LoginClient> get values => _$values;
}

mixin _$LoginClientMixin {
  // ignore: non_constant_identifier_names
  _$LoginClientMeta get LoginClient => const _$LoginClientMeta();
}

Serializer<LoginClient> _$loginClientSerializer = _$LoginClientSerializer();

class _$LoginClientSerializer implements PrimitiveSerializer<LoginClient> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'app': 'app',
    'console': 'console',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'app': 'app',
    'console': 'console',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[LoginClient];
  @override
  final String wireName = 'LoginClient';

  @override
  Object serialize(
    Serializers serializers,
    LoginClient object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  LoginClient deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => LoginClient.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
