//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'session_revoked_payload.g.dart';

/// SessionRevokedPayload
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
@BuiltValue()
abstract class SessionRevokedPayload
    implements Built<SessionRevokedPayload, SessionRevokedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'reason')
  SessionRevokedPayloadReasonEnum get reason;
  // enum reasonEnum {  logout,  admin,  device_revoked,  refresh_reuse,  user_disabled,  expired,  inactive,  step_up_failed,  trusted_header_mismatch,  };

  SessionRevokedPayload._();

  factory SessionRevokedPayload([
    void updates(SessionRevokedPayloadBuilder b),
  ]) = _$SessionRevokedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SessionRevokedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SessionRevokedPayload> get serializer =>
      _$SessionRevokedPayloadSerializer();
}

class _$SessionRevokedPayloadSerializer
    implements PrimitiveSerializer<SessionRevokedPayload> {
  @override
  final Iterable<Type> types = const [
    SessionRevokedPayload,
    _$SessionRevokedPayload,
  ];

  @override
  final String wireName = r'SessionRevokedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SessionRevokedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(SessionRevokedPayloadReasonEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SessionRevokedPayload object, {
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
    required SessionRevokedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SessionRevokedPayloadReasonEnum),
          ) as SessionRevokedPayloadReasonEnum;
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
  SessionRevokedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SessionRevokedPayloadBuilder();
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

class SessionRevokedPayloadReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'logout')
  static const SessionRevokedPayloadReasonEnum logout =
      _$sessionRevokedPayloadReasonEnum_logout;
  @BuiltValueEnumConst(wireName: r'admin')
  static const SessionRevokedPayloadReasonEnum admin =
      _$sessionRevokedPayloadReasonEnum_admin;
  @BuiltValueEnumConst(wireName: r'device_revoked')
  static const SessionRevokedPayloadReasonEnum deviceRevoked =
      _$sessionRevokedPayloadReasonEnum_deviceRevoked;
  @BuiltValueEnumConst(wireName: r'refresh_reuse')
  static const SessionRevokedPayloadReasonEnum refreshReuse =
      _$sessionRevokedPayloadReasonEnum_refreshReuse;
  @BuiltValueEnumConst(wireName: r'user_disabled')
  static const SessionRevokedPayloadReasonEnum userDisabled =
      _$sessionRevokedPayloadReasonEnum_userDisabled;
  @BuiltValueEnumConst(wireName: r'expired')
  static const SessionRevokedPayloadReasonEnum expired =
      _$sessionRevokedPayloadReasonEnum_expired;
  @BuiltValueEnumConst(wireName: r'inactive')
  static const SessionRevokedPayloadReasonEnum inactive =
      _$sessionRevokedPayloadReasonEnum_inactive;
  @BuiltValueEnumConst(wireName: r'step_up_failed')
  static const SessionRevokedPayloadReasonEnum stepUpFailed =
      _$sessionRevokedPayloadReasonEnum_stepUpFailed;
  @BuiltValueEnumConst(wireName: r'trusted_header_mismatch')
  static const SessionRevokedPayloadReasonEnum trustedHeaderMismatch =
      _$sessionRevokedPayloadReasonEnum_trustedHeaderMismatch;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SessionRevokedPayloadReasonEnum unknownDefaultOpenApi =
      _$sessionRevokedPayloadReasonEnum_unknownDefaultOpenApi;

  static Serializer<SessionRevokedPayloadReasonEnum> get serializer =>
      _$sessionRevokedPayloadReasonEnumSerializer;

  const SessionRevokedPayloadReasonEnum._(String name) : super(name);

  static BuiltSet<SessionRevokedPayloadReasonEnum> get values =>
      _$sessionRevokedPayloadReasonEnumValues;
  static SessionRevokedPayloadReasonEnum valueOf(String name) =>
      _$sessionRevokedPayloadReasonEnumValueOf(name);
}
