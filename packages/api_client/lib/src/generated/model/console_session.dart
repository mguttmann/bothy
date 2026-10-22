//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/console_session_view_as.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'console_session.g.dart';

/// ConsoleSession
///
/// Properties:
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [displayName]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [idleTimeoutSeconds]
/// * [stepUpAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stepUpMethod]
/// * [stepUpValidUntil] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stepUpMethodsAvailable]
/// * [viewAs]
@BuiltValue()
abstract class ConsoleSession
    implements Built<ConsoleSession, ConsoleSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  @BuiltValueField(wireName: r'idle_timeout_seconds')
  int get idleTimeoutSeconds;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'step_up_at')
  DateTime? get stepUpAt;

  @BuiltValueField(wireName: r'step_up_method')
  ConsoleSessionStepUpMethodEnum? get stepUpMethod;
  // enum stepUpMethodEnum {  passkey,  password_totp,  oidc_reauth,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'step_up_valid_until')
  DateTime? get stepUpValidUntil;

  @BuiltValueField(wireName: r'step_up_methods_available')
  BuiltList<ConsoleSessionStepUpMethodsAvailableEnum>
  get stepUpMethodsAvailable;
  // enum stepUpMethodsAvailableEnum {  passkey,  oidc_reauth,  password_totp,  };

  @BuiltValueField(wireName: r'view_as')
  ConsoleSessionViewAs? get viewAs;

  ConsoleSession._();

  factory ConsoleSession([void updates(ConsoleSessionBuilder b)]) =
      _$ConsoleSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ConsoleSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ConsoleSession> get serializer =>
      _$ConsoleSessionSerializer();
}

class _$ConsoleSessionSerializer
    implements PrimitiveSerializer<ConsoleSession> {
  @override
  final Iterable<Type> types = const [ConsoleSession, _$ConsoleSession];

  @override
  final String wireName = r'ConsoleSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ConsoleSession object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'idle_timeout_seconds';
    yield serializers.serialize(
      object.idleTimeoutSeconds,
      specifiedType: const FullType(int),
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
        specifiedType: const FullType(ConsoleSessionStepUpMethodEnum),
      );
    }
    if (object.stepUpValidUntil != null) {
      yield r'step_up_valid_until';
      yield serializers.serialize(
        object.stepUpValidUntil,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'step_up_methods_available';
    yield serializers.serialize(
      object.stepUpMethodsAvailable,
      specifiedType: const FullType(BuiltList, [
        FullType(ConsoleSessionStepUpMethodsAvailableEnum),
      ]),
    );
    if (object.viewAs != null) {
      yield r'view_as';
      yield serializers.serialize(
        object.viewAs,
        specifiedType: const FullType(ConsoleSessionViewAs),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ConsoleSession object, {
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
    required ConsoleSessionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        case r'idle_timeout_seconds':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.idleTimeoutSeconds = valueDes;
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
            specifiedType: const FullType.nullable(
              ConsoleSessionStepUpMethodEnum,
            ),
          ) as ConsoleSessionStepUpMethodEnum?;
          if (valueDes == null) continue;
          result.stepUpMethod = valueDes;
          break;
        case r'step_up_valid_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.stepUpValidUntil = valueDes;
          break;
        case r'step_up_methods_available':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(ConsoleSessionStepUpMethodsAvailableEnum),
            ]),
          ) as BuiltList<ConsoleSessionStepUpMethodsAvailableEnum>;
          result.stepUpMethodsAvailable.replace(valueDes);
          break;
        case r'view_as':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ConsoleSessionViewAs),
          ) as ConsoleSessionViewAs?;
          if (valueDes == null) continue;
          result.viewAs.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ConsoleSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ConsoleSessionBuilder();
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

class ConsoleSessionStepUpMethodEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'passkey')
  static const ConsoleSessionStepUpMethodEnum passkey =
      _$consoleSessionStepUpMethodEnum_passkey;
  @BuiltValueEnumConst(wireName: r'password_totp')
  static const ConsoleSessionStepUpMethodEnum passwordTotp =
      _$consoleSessionStepUpMethodEnum_passwordTotp;
  @BuiltValueEnumConst(wireName: r'oidc_reauth')
  static const ConsoleSessionStepUpMethodEnum oidcReauth =
      _$consoleSessionStepUpMethodEnum_oidcReauth;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ConsoleSessionStepUpMethodEnum unknownDefaultOpenApi =
      _$consoleSessionStepUpMethodEnum_unknownDefaultOpenApi;

  static Serializer<ConsoleSessionStepUpMethodEnum> get serializer =>
      _$consoleSessionStepUpMethodEnumSerializer;

  const ConsoleSessionStepUpMethodEnum._(String name) : super(name);

  static BuiltSet<ConsoleSessionStepUpMethodEnum> get values =>
      _$consoleSessionStepUpMethodEnumValues;
  static ConsoleSessionStepUpMethodEnum valueOf(String name) =>
      _$consoleSessionStepUpMethodEnumValueOf(name);
}

class ConsoleSessionStepUpMethodsAvailableEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'passkey')
  static const ConsoleSessionStepUpMethodsAvailableEnum passkey =
      _$consoleSessionStepUpMethodsAvailableEnum_passkey;
  @BuiltValueEnumConst(wireName: r'oidc_reauth')
  static const ConsoleSessionStepUpMethodsAvailableEnum oidcReauth =
      _$consoleSessionStepUpMethodsAvailableEnum_oidcReauth;
  @BuiltValueEnumConst(wireName: r'password_totp')
  static const ConsoleSessionStepUpMethodsAvailableEnum passwordTotp =
      _$consoleSessionStepUpMethodsAvailableEnum_passwordTotp;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ConsoleSessionStepUpMethodsAvailableEnum unknownDefaultOpenApi =
      _$consoleSessionStepUpMethodsAvailableEnum_unknownDefaultOpenApi;

  static Serializer<ConsoleSessionStepUpMethodsAvailableEnum> get serializer =>
      _$consoleSessionStepUpMethodsAvailableEnumSerializer;

  const ConsoleSessionStepUpMethodsAvailableEnum._(String name) : super(name);

  static BuiltSet<ConsoleSessionStepUpMethodsAvailableEnum> get values =>
      _$consoleSessionStepUpMethodsAvailableEnumValues;
  static ConsoleSessionStepUpMethodsAvailableEnum valueOf(String name) =>
      _$consoleSessionStepUpMethodsAvailableEnumValueOf(name);
}
