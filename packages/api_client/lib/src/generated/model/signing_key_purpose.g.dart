// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'signing_key_purpose.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SigningKeyPurpose _$accessToken = const SigningKeyPurpose._(
  'accessToken',
);
const SigningKeyPurpose _$invocation = const SigningKeyPurpose._('invocation');
const SigningKeyPurpose _$terminal = const SigningKeyPurpose._('terminal');
const SigningKeyPurpose _$webhookOut = const SigningKeyPurpose._('webhookOut');
const SigningKeyPurpose _$auditCheckpoint = const SigningKeyPurpose._(
  'auditCheckpoint',
);
const SigningKeyPurpose _$unknownDefaultOpenApi = const SigningKeyPurpose._(
  'unknownDefaultOpenApi',
);

SigningKeyPurpose _$valueOf(String name) {
  switch (name) {
    case 'accessToken':
      return _$accessToken;
    case 'invocation':
      return _$invocation;
    case 'terminal':
      return _$terminal;
    case 'webhookOut':
      return _$webhookOut;
    case 'auditCheckpoint':
      return _$auditCheckpoint;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<SigningKeyPurpose> _$values = BuiltSet<SigningKeyPurpose>(
  const <SigningKeyPurpose>[
    _$accessToken,
    _$invocation,
    _$terminal,
    _$webhookOut,
    _$auditCheckpoint,
    _$unknownDefaultOpenApi,
  ],
);

class _$SigningKeyPurposeMeta {
  const _$SigningKeyPurposeMeta();
  SigningKeyPurpose get accessToken => _$accessToken;
  SigningKeyPurpose get invocation => _$invocation;
  SigningKeyPurpose get terminal => _$terminal;
  SigningKeyPurpose get webhookOut => _$webhookOut;
  SigningKeyPurpose get auditCheckpoint => _$auditCheckpoint;
  SigningKeyPurpose get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  SigningKeyPurpose valueOf(String name) => _$valueOf(name);
  BuiltSet<SigningKeyPurpose> get values => _$values;
}

mixin _$SigningKeyPurposeMixin {
  // ignore: non_constant_identifier_names
  _$SigningKeyPurposeMeta get SigningKeyPurpose =>
      const _$SigningKeyPurposeMeta();
}

Serializer<SigningKeyPurpose> _$signingKeyPurposeSerializer =
    _$SigningKeyPurposeSerializer();

class _$SigningKeyPurposeSerializer
    implements PrimitiveSerializer<SigningKeyPurpose> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'accessToken': 'access_token',
    'invocation': 'invocation',
    'terminal': 'terminal',
    'webhookOut': 'webhook_out',
    'auditCheckpoint': 'audit_checkpoint',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'access_token': 'accessToken',
    'invocation': 'invocation',
    'terminal': 'terminal',
    'webhook_out': 'webhookOut',
    'audit_checkpoint': 'auditCheckpoint',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SigningKeyPurpose];
  @override
  final String wireName = 'SigningKeyPurpose';

  @override
  Object serialize(
    Serializers serializers,
    SigningKeyPurpose object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SigningKeyPurpose deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SigningKeyPurpose.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
