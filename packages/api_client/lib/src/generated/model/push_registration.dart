//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/push_environment.dart';
import 'package:api_client/src/generated/model/push_provider.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'push_registration.g.dart';

/// PushRegistration
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
@BuiltValue(instantiable: false)
abstract class PushRegistration {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  @BuiltValueField(wireName: r'provider')
  PushProvider get provider;
  // enum providerEnum {  apns,  fcm,  unifiedpush,  wns,  };

  @BuiltValueField(wireName: r'environment')
  PushEnvironment get environment;
  // enum environmentEnum {  production,  sandbox,  };

  @BuiltValueField(wireName: r'app_id')
  String get appId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'disabled_at')
  DateTime? get disabledAt;

  @BuiltValueField(wireName: r'disabled_reason')
  String? get disabledReason;

  @BuiltValueSerializer(custom: true)
  static Serializer<PushRegistration> get serializer =>
      _$PushRegistrationSerializer();
}

class _$PushRegistrationSerializer
    implements PrimitiveSerializer<PushRegistration> {
  @override
  final Iterable<Type> types = const [PushRegistration];

  @override
  final String wireName = r'PushRegistration';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PushRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(PushProvider),
    );
    yield r'environment';
    yield serializers.serialize(
      object.environment,
      specifiedType: const FullType(PushEnvironment),
    );
    yield r'app_id';
    yield serializers.serialize(
      object.appId,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.disabledAt != null) {
      yield r'disabled_at';
      yield serializers.serialize(
        object.disabledAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.disabledReason != null) {
      yield r'disabled_reason';
      yield serializers.serialize(
        object.disabledReason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PushRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  @override
  PushRegistration deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return serializers.deserialize(
      serialized,
      specifiedType: FullType($PushRegistration),
    ) as $PushRegistration;
  }
}

/// a concrete implementation of [PushRegistration], since [PushRegistration] is not instantiable
@BuiltValue(instantiable: true)
abstract class $PushRegistration
    implements
        PushRegistration,
        Built<$PushRegistration, $PushRegistrationBuilder> {
  $PushRegistration._();

  factory $PushRegistration([
    void Function($PushRegistrationBuilder)? updates,
  ]) = _$$PushRegistration;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults($PushRegistrationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<$PushRegistration> get serializer =>
      _$$PushRegistrationSerializer();
}

class _$$PushRegistrationSerializer
    implements PrimitiveSerializer<$PushRegistration> {
  @override
  final Iterable<Type> types = const [$PushRegistration, _$$PushRegistration];

  @override
  final String wireName = r'$PushRegistration';

  @override
  Object serialize(
    Serializers serializers,
    $PushRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return serializers.serialize(
      object,
      specifiedType: FullType(PushRegistration),
    )!;
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required PushRegistrationBuilder result,
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
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PushProvider),
          ) as PushProvider;
          result.provider = valueDes;
          break;
        case r'environment':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PushEnvironment),
          ) as PushEnvironment;
          result.environment = valueDes;
          break;
        case r'app_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appId = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'disabled_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.disabledAt = valueDes;
          break;
        case r'disabled_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.disabledReason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  $PushRegistration deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = $PushRegistrationBuilder();
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
