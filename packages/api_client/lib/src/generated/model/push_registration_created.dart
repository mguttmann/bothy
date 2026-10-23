//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/push_environment.dart';
import 'package:api_client/src/generated/model/push_provider.dart';
import 'package:api_client/src/generated/model/push_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'push_registration_created.g.dart';

/// A new push registration with its preview token, which is shown exactly once and never again (13, 6.4).
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [provider]
/// * [environment]
/// * [appId]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [disabledAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [disabledReason]
/// * [previewToken] - Preview token of the registration (`bothy_pvw_...`), shown exactly once. The app keeps it in the store it shares with its notification extension.
@BuiltValue()
abstract class PushRegistrationCreated
    implements
        PushRegistration,
        Built<PushRegistrationCreated, PushRegistrationCreatedBuilder> {
  /// Preview token of the registration (`bothy_pvw_...`), shown exactly once. The app keeps it in the store it shares with its notification extension.
  @BuiltValueField(wireName: r'preview_token')
  String get previewToken;

  PushRegistrationCreated._();

  factory PushRegistrationCreated([
    void updates(PushRegistrationCreatedBuilder b),
  ]) = _$PushRegistrationCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PushRegistrationCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PushRegistrationCreated> get serializer =>
      _$PushRegistrationCreatedSerializer();
}

class _$PushRegistrationCreatedSerializer
    implements PrimitiveSerializer<PushRegistrationCreated> {
  @override
  final Iterable<Type> types = const [
    PushRegistrationCreated,
    _$PushRegistrationCreated,
  ];

  @override
  final String wireName = r'PushRegistrationCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PushRegistrationCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.disabledReason != null) {
      yield r'disabled_reason';
      yield serializers.serialize(
        object.disabledReason,
        specifiedType: const FullType(String),
      );
    }
    yield r'environment';
    yield serializers.serialize(
      object.environment,
      specifiedType: const FullType(PushEnvironment),
    );
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(PushProvider),
    );
    yield r'app_id';
    yield serializers.serialize(
      object.appId,
      specifiedType: const FullType(String),
    );
    yield r'preview_token';
    yield serializers.serialize(
      object.previewToken,
      specifiedType: const FullType(String),
    );
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
    if (object.disabledAt != null) {
      yield r'disabled_at';
      yield serializers.serialize(
        object.disabledAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PushRegistrationCreated object, {
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
    required PushRegistrationCreatedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'disabled_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.disabledReason = valueDes;
          break;
        case r'environment':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PushEnvironment),
          ) as PushEnvironment;
          result.environment = valueDes;
          break;
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PushProvider),
          ) as PushProvider;
          result.provider = valueDes;
          break;
        case r'app_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appId = valueDes;
          break;
        case r'preview_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.previewToken = valueDes;
          break;
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'disabled_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.disabledAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PushRegistrationCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PushRegistrationCreatedBuilder();
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
