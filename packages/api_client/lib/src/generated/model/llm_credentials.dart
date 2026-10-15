//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'llm_credentials.g.dart';

/// Owner declaration of the LLM login on a host (24, 7.2). `personal` needs `credential_holder_id` (I6).
class LlmCredentials extends EnumClass {
  @BuiltValueEnumConst(wireName: r'none')
  static const LlmCredentials none = _$none;
  @BuiltValueEnumConst(wireName: r'personal')
  static const LlmCredentials personal = _$personal;
  @BuiltValueEnumConst(wireName: r'organization')
  static const LlmCredentials organization = _$organization;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const LlmCredentials unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<LlmCredentials> get serializer =>
      _$llmCredentialsSerializer;

  const LlmCredentials._(String name) : super(name);

  static BuiltSet<LlmCredentials> get values => _$values;
  static LlmCredentials valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class LlmCredentialsMixin = Object with _$LlmCredentialsMixin;
