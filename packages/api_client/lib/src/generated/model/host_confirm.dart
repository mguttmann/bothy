//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host_session_user_assign.dart';
import 'package:api_client/src/generated/model/llm_credentials.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_confirm.g.dart';

/// HostConfirm
///
/// Properties:
/// * [fingerprintPrefix] - First `terminal.fingerprint_confirm_chars` characters of the fingerprint without separators, case-insensitive.
/// * [llmCredentials]
/// * [credentialHolderId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [declaredServerHost]
/// * [sessionUsers] - Session user per person (ruling T12); a person without one gets no terminal on the host (SR12).
/// * [reason]
@BuiltValue()
abstract class HostConfirm implements Built<HostConfirm, HostConfirmBuilder> {
  /// First `terminal.fingerprint_confirm_chars` characters of the fingerprint without separators, case-insensitive.
  @BuiltValueField(wireName: r'fingerprint_prefix')
  String get fingerprintPrefix;

  @BuiltValueField(wireName: r'llm_credentials')
  LlmCredentials get llmCredentials;
  // enum llmCredentialsEnum {  none,  personal,  organization,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  @BuiltValueField(wireName: r'declared_server_host')
  bool get declaredServerHost;

  /// Session user per person (ruling T12); a person without one gets no terminal on the host (SR12).
  @BuiltValueField(wireName: r'session_users')
  BuiltList<HostSessionUserAssign>? get sessionUsers;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  HostConfirm._();

  factory HostConfirm([void updates(HostConfirmBuilder b)]) = _$HostConfirm;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostConfirmBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostConfirm> get serializer => _$HostConfirmSerializer();
}

class _$HostConfirmSerializer implements PrimitiveSerializer<HostConfirm> {
  @override
  final Iterable<Type> types = const [HostConfirm, _$HostConfirm];

  @override
  final String wireName = r'HostConfirm';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostConfirm object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'fingerprint_prefix';
    yield serializers.serialize(
      object.fingerprintPrefix,
      specifiedType: const FullType(String),
    );
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
    yield r'declared_server_host';
    yield serializers.serialize(
      object.declaredServerHost,
      specifiedType: const FullType(bool),
    );
    if (object.sessionUsers != null) {
      yield r'session_users';
      yield serializers.serialize(
        object.sessionUsers,
        specifiedType: const FullType(BuiltList, [
          FullType(HostSessionUserAssign),
        ]),
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
    HostConfirm object, {
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
    required HostConfirmBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'fingerprint_prefix':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.fingerprintPrefix = valueDes;
          break;
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
        case r'declared_server_host':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.declaredServerHost = valueDes;
          break;
        case r'session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(HostSessionUserAssign),
            ]),
          ) as BuiltList<HostSessionUserAssign>?;
          if (valueDes == null) continue;
          result.sessionUsers.replace(valueDes);
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
  HostConfirm deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostConfirmBuilder();
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
