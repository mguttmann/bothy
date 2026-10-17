//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'session.g.dart';

/// Session
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [authMethod]
/// * [clientComponent]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stepUpAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stepUpMethod]
/// * [lastSeenAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [current] - True for the session of this request. Set only in REST responses.
@BuiltValue()
abstract class Session implements Built<Session, SessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  @BuiltValueField(wireName: r'auth_method')
  SessionAuthMethodEnum get authMethod;
  // enum authMethodEnum {  oidc,  trusted_header,  passkey,  password_totp,  enrollment_code,  device_pairing,  };

  @BuiltValueField(wireName: r'client_component')
  SessionClientComponentEnum? get clientComponent;
  // enum clientComponentEnum {  app,  share_ext,  console,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'step_up_at')
  DateTime? get stepUpAt;

  @BuiltValueField(wireName: r'step_up_method')
  SessionStepUpMethodEnum? get stepUpMethod;
  // enum stepUpMethodEnum {  passkey,  device_key,  password_totp,  oidc_reauth,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_seen_at')
  DateTime? get lastSeenAt;

  /// True for the session of this request. Set only in REST responses.
  @BuiltValueField(wireName: r'current')
  bool? get current;

  Session._();

  factory Session([void updates(SessionBuilder b)]) = _$Session;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Session> get serializer => _$SessionSerializer();
}

class _$SessionSerializer implements PrimitiveSerializer<Session> {
  @override
  final Iterable<Type> types = const [Session, _$Session];

  @override
  final String wireName = r'Session';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Session object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
    yield r'auth_method';
    yield serializers.serialize(
      object.authMethod,
      specifiedType: const FullType(SessionAuthMethodEnum),
    );
    if (object.clientComponent != null) {
      yield r'client_component';
      yield serializers.serialize(
        object.clientComponent,
        specifiedType: const FullType(SessionClientComponentEnum),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.stepUpAt != null) {
      yield r'step_up_at';
      yield serializers.serialize(
        object.stepUpAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.stepUpMethod != null) {
      yield r'step_up_method';
      yield serializers.serialize(
        object.stepUpMethod,
        specifiedType: const FullType(SessionStepUpMethodEnum),
      );
    }
    if (object.lastSeenAt != null) {
      yield r'last_seen_at';
      yield serializers.serialize(
        object.lastSeenAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.current != null) {
      yield r'current';
      yield serializers.serialize(
        object.current,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Session object, {
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
    required SessionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'auth_method':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SessionAuthMethodEnum),
          ) as SessionAuthMethodEnum;
          result.authMethod = valueDes;
          break;
        case r'client_component':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(SessionClientComponentEnum),
          ) as SessionClientComponentEnum?;
          if (valueDes == null) continue;
          result.clientComponent = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        case r'step_up_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.stepUpAt = valueDes;
          break;
        case r'step_up_method':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(SessionStepUpMethodEnum),
          ) as SessionStepUpMethodEnum?;
          if (valueDes == null) continue;
          result.stepUpMethod = valueDes;
          break;
        case r'last_seen_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.lastSeenAt = valueDes;
          break;
        case r'current':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.current = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Session deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SessionBuilder();
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

class SessionAuthMethodEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'oidc')
  static const SessionAuthMethodEnum oidc = _$sessionAuthMethodEnum_oidc;
  @BuiltValueEnumConst(wireName: r'trusted_header')
  static const SessionAuthMethodEnum trustedHeader =
      _$sessionAuthMethodEnum_trustedHeader;
  @BuiltValueEnumConst(wireName: r'passkey')
  static const SessionAuthMethodEnum passkey = _$sessionAuthMethodEnum_passkey;
  @BuiltValueEnumConst(wireName: r'password_totp')
  static const SessionAuthMethodEnum passwordTotp =
      _$sessionAuthMethodEnum_passwordTotp;
  @BuiltValueEnumConst(wireName: r'enrollment_code')
  static const SessionAuthMethodEnum enrollmentCode =
      _$sessionAuthMethodEnum_enrollmentCode;
  @BuiltValueEnumConst(wireName: r'device_pairing')
  static const SessionAuthMethodEnum devicePairing =
      _$sessionAuthMethodEnum_devicePairing;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SessionAuthMethodEnum unknownDefaultOpenApi =
      _$sessionAuthMethodEnum_unknownDefaultOpenApi;

  static Serializer<SessionAuthMethodEnum> get serializer =>
      _$sessionAuthMethodEnumSerializer;

  const SessionAuthMethodEnum._(String name) : super(name);

  static BuiltSet<SessionAuthMethodEnum> get values =>
      _$sessionAuthMethodEnumValues;
  static SessionAuthMethodEnum valueOf(String name) =>
      _$sessionAuthMethodEnumValueOf(name);
}

class SessionClientComponentEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'app')
  static const SessionClientComponentEnum app =
      _$sessionClientComponentEnum_app;
  @BuiltValueEnumConst(wireName: r'share_ext')
  static const SessionClientComponentEnum shareExt =
      _$sessionClientComponentEnum_shareExt;
  @BuiltValueEnumConst(wireName: r'console')
  static const SessionClientComponentEnum console =
      _$sessionClientComponentEnum_console;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SessionClientComponentEnum unknownDefaultOpenApi =
      _$sessionClientComponentEnum_unknownDefaultOpenApi;

  static Serializer<SessionClientComponentEnum> get serializer =>
      _$sessionClientComponentEnumSerializer;

  const SessionClientComponentEnum._(String name) : super(name);

  static BuiltSet<SessionClientComponentEnum> get values =>
      _$sessionClientComponentEnumValues;
  static SessionClientComponentEnum valueOf(String name) =>
      _$sessionClientComponentEnumValueOf(name);
}

class SessionStepUpMethodEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'passkey')
  static const SessionStepUpMethodEnum passkey =
      _$sessionStepUpMethodEnum_passkey;
  @BuiltValueEnumConst(wireName: r'device_key')
  static const SessionStepUpMethodEnum deviceKey =
      _$sessionStepUpMethodEnum_deviceKey;
  @BuiltValueEnumConst(wireName: r'password_totp')
  static const SessionStepUpMethodEnum passwordTotp =
      _$sessionStepUpMethodEnum_passwordTotp;
  @BuiltValueEnumConst(wireName: r'oidc_reauth')
  static const SessionStepUpMethodEnum oidcReauth =
      _$sessionStepUpMethodEnum_oidcReauth;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SessionStepUpMethodEnum unknownDefaultOpenApi =
      _$sessionStepUpMethodEnum_unknownDefaultOpenApi;

  static Serializer<SessionStepUpMethodEnum> get serializer =>
      _$sessionStepUpMethodEnumSerializer;

  const SessionStepUpMethodEnum._(String name) : super(name);

  static BuiltSet<SessionStepUpMethodEnum> get values =>
      _$sessionStepUpMethodEnumValues;
  static SessionStepUpMethodEnum valueOf(String name) =>
      _$sessionStepUpMethodEnumValueOf(name);
}
