//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/llm_credentials.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_llm_credentials_set.g.dart';

/// HostLlmCredentialsSet
///
/// Properties:
/// * [llmCredentials]
/// * [credentialHolderId] - Required exactly with `personal`.
/// * [reason]
@BuiltValue()
abstract class HostLlmCredentialsSet
    implements Built<HostLlmCredentialsSet, HostLlmCredentialsSetBuilder> {
  @BuiltValueField(wireName: r'llm_credentials')
  LlmCredentials get llmCredentials;
  // enum llmCredentialsEnum {  none,  personal,  organization,  };

  /// Required exactly with `personal`.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  HostLlmCredentialsSet._();

  factory HostLlmCredentialsSet([
    void updates(HostLlmCredentialsSetBuilder b),
  ]) = _$HostLlmCredentialsSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostLlmCredentialsSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostLlmCredentialsSet> get serializer =>
      _$HostLlmCredentialsSetSerializer();
}

class _$HostLlmCredentialsSetSerializer
    implements PrimitiveSerializer<HostLlmCredentialsSet> {
  @override
  final Iterable<Type> types = const [
    HostLlmCredentialsSet,
    _$HostLlmCredentialsSet,
  ];

  @override
  final String wireName = r'HostLlmCredentialsSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostLlmCredentialsSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'llm_credentials';
    yield serializers.serialize(
      object.llmCredentials,
      specifiedType: const FullType(LlmCredentials),
    );
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostLlmCredentialsSet object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required HostLlmCredentialsSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'llm_credentials':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LlmCredentials),
          ) as LlmCredentials;
          result.llmCredentials = valueDes;
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.credentialHolderId = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostLlmCredentialsSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostLlmCredentialsSetBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}
