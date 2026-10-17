// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'llm_credentials.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const LlmCredentials _$none = const LlmCredentials._('none');
const LlmCredentials _$personal = const LlmCredentials._('personal');
const LlmCredentials _$organization = const LlmCredentials._('organization');
const LlmCredentials _$unknownDefaultOpenApi = const LlmCredentials._(
  'unknownDefaultOpenApi',
);

LlmCredentials _$valueOf(String name) {
  switch (name) {
    case 'none':
      return _$none;
    case 'personal':
      return _$personal;
    case 'organization':
      return _$organization;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<LlmCredentials> _$values = BuiltSet<LlmCredentials>(
  const <LlmCredentials>[
    _$none,
    _$personal,
    _$organization,
    _$unknownDefaultOpenApi,
  ],
);

class _$LlmCredentialsMeta {
  const _$LlmCredentialsMeta();
  LlmCredentials get none => _$none;
  LlmCredentials get personal => _$personal;
  LlmCredentials get organization => _$organization;
  LlmCredentials get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  LlmCredentials valueOf(String name) => _$valueOf(name);
  BuiltSet<LlmCredentials> get values => _$values;
}

mixin _$LlmCredentialsMixin {
  // ignore: non_constant_identifier_names
  _$LlmCredentialsMeta get LlmCredentials => const _$LlmCredentialsMeta();
}

Serializer<LlmCredentials> _$llmCredentialsSerializer =
    _$LlmCredentialsSerializer();

class _$LlmCredentialsSerializer
    implements PrimitiveSerializer<LlmCredentials> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'none': 'none',
    'personal': 'personal',
    'organization': 'organization',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'none': 'none',
    'personal': 'personal',
    'organization': 'organization',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[LlmCredentials];
  @override
  final String wireName = 'LlmCredentials';

  @override
  Object serialize(
    Serializers serializers,
    LlmCredentials object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  LlmCredentials deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => LlmCredentials.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
