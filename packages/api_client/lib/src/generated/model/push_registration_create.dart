//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/push_environment.dart';
import 'package:api_client/src/generated/model/push_provider.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'push_registration_create.g.dart';

/// PushRegistrationCreate
///
/// Properties:
/// * [provider]
/// * [token] - Push token of the provider, at most `push.registration_field_max_bytes` bytes (`too_long` at `/token`). Never returned. An `apns` token is an even number of hexadecimal digits and is stored in lower case; an `fcm` token is printable ASCII without white space; otherwise `pattern_mismatch` at `/token`.
/// * [environment]
/// * [appId] - Bundle ID or package of the app, at most `push.registration_field_max_bytes` bytes (`too_long` at `/app_id`). For `apns` it must be one of `push.apns.topics` (`invalid` at `/app_id`). `apns` needs a device with platform `ios` or `macos`, `fcm` one with `android` (`invalid` at `/provider`); `sandbox` exists only for `apns` (`invalid` at `/environment`); an absent `environment` is `production`.
@BuiltValue()
abstract class PushRegistrationCreate
    implements Built<PushRegistrationCreate, PushRegistrationCreateBuilder> {
  @BuiltValueField(wireName: r'provider')
  PushProvider get provider;
  // enum providerEnum {  apns,  fcm,  unifiedpush,  wns,  };

  /// Push token of the provider, at most `push.registration_field_max_bytes` bytes (`too_long` at `/token`). Never returned. An `apns` token is an even number of hexadecimal digits and is stored in lower case; an `fcm` token is printable ASCII without white space; otherwise `pattern_mismatch` at `/token`.
  @BuiltValueField(wireName: r'token')
  String get token;

  @BuiltValueField(wireName: r'environment')
  PushEnvironment? get environment;
  // enum environmentEnum {  production,  sandbox,  };

  /// Bundle ID or package of the app, at most `push.registration_field_max_bytes` bytes (`too_long` at `/app_id`). For `apns` it must be one of `push.apns.topics` (`invalid` at `/app_id`). `apns` needs a device with platform `ios` or `macos`, `fcm` one with `android` (`invalid` at `/provider`); `sandbox` exists only for `apns` (`invalid` at `/environment`); an absent `environment` is `production`.
  @BuiltValueField(wireName: r'app_id')
  String get appId;

  PushRegistrationCreate._();

  factory PushRegistrationCreate([
    void updates(PushRegistrationCreateBuilder b),
  ]) = _$PushRegistrationCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PushRegistrationCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PushRegistrationCreate> get serializer =>
      _$PushRegistrationCreateSerializer();
}

class _$PushRegistrationCreateSerializer
    implements PrimitiveSerializer<PushRegistrationCreate> {
  @override
  final Iterable<Type> types = const [
    PushRegistrationCreate,
    _$PushRegistrationCreate,
  ];

  @override
  final String wireName = r'PushRegistrationCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PushRegistrationCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(PushProvider),
    );
    yield r'token';
    yield serializers.serialize(
      object.token,
      specifiedType: const FullType(String),
    );
    if (object.environment != null) {
      yield r'environment';
      yield serializers.serialize(
        object.environment,
        specifiedType: const FullType(PushEnvironment),
      );
    }
    yield r'app_id';
    yield serializers.serialize(
      object.appId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PushRegistrationCreate object, {
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
    required PushRegistrationCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PushProvider),
          ) as PushProvider;
          result.provider = valueDes;
          break;
        case r'token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.token = valueDes;
          break;
        case r'environment':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(PushEnvironment),
          ) as PushEnvironment?;
          if (valueDes == null) continue;
          result.environment = valueDes;
          break;
        case r'app_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PushRegistrationCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PushRegistrationCreateBuilder();
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
