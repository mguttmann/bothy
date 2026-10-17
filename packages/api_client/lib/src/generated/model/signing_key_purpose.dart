//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'signing_key_purpose.g.dart';

/// Purpose of a server signing key (03 `server_signing_keys.purpose`). `terminal` signs terminal session tokens (24 section 5.3); `audit_checkpoint` signs audit checkpoints (19 SEC-AUD-7).
class SigningKeyPurpose extends EnumClass {
  @BuiltValueEnumConst(wireName: r'access_token')
  static const SigningKeyPurpose accessToken = _$accessToken;
  @BuiltValueEnumConst(wireName: r'invocation')
  static const SigningKeyPurpose invocation = _$invocation;
  @BuiltValueEnumConst(wireName: r'terminal')
  static const SigningKeyPurpose terminal = _$terminal;
  @BuiltValueEnumConst(wireName: r'webhook_out')
  static const SigningKeyPurpose webhookOut = _$webhookOut;
  @BuiltValueEnumConst(wireName: r'audit_checkpoint')
  static const SigningKeyPurpose auditCheckpoint = _$auditCheckpoint;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SigningKeyPurpose unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<SigningKeyPurpose> get serializer =>
      _$signingKeyPurposeSerializer;

  const SigningKeyPurpose._(String name) : super(name);

  static BuiltSet<SigningKeyPurpose> get values => _$values;
  static SigningKeyPurpose valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class SigningKeyPurposeMixin = Object with _$SigningKeyPurposeMixin;
