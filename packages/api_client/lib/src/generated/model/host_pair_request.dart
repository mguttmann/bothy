//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/host_platform.dart';
import 'package:api_client/src/generated/model/host_reported_session_user.dart';
import 'package:api_client/src/generated/model/host_profile.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_pair_request.g.dart';

/// HostPairRequest
///
/// Properties:
/// * [pairingCode]
/// * [noisePublicKey] - Static X25519 public key, 32 bytes base64url without padding.
/// * [authPublicJwk] - Ed25519 public key as JWK (`kty = OKP`, `crv = Ed25519`); a private member is rejected.
/// * [platform]
/// * [osVersion]
/// * [hostVersion]
/// * [profiles]
/// * [sessionUsers] - The session users of `host_service.session_users` with the display names of their working directories (24 section 2.2).
@BuiltValue()
abstract class HostPairRequest
    implements Built<HostPairRequest, HostPairRequestBuilder> {
  @BuiltValueField(wireName: r'pairing_code')
  String get pairingCode;

  /// Static X25519 public key, 32 bytes base64url without padding.
  @BuiltValueField(wireName: r'noise_public_key')
  String get noisePublicKey;

  /// Ed25519 public key as JWK (`kty = OKP`, `crv = Ed25519`); a private member is rejected.
  @BuiltValueField(wireName: r'auth_public_jwk')
  BuiltMap<String, JsonObject?> get authPublicJwk;

  @BuiltValueField(wireName: r'platform')
  HostPlatform get platform;
  // enum platformEnum {  macos,  linux,  windows,  };

  @BuiltValueField(wireName: r'os_version')
  String? get osVersion;

  @BuiltValueField(wireName: r'host_version')
  String get hostVersion;

  @BuiltValueField(wireName: r'profiles')
  BuiltList<HostProfile> get profiles;

  /// The session users of `host_service.session_users` with the display names of their working directories (24 section 2.2).
  @BuiltValueField(wireName: r'session_users')
  BuiltList<HostReportedSessionUser> get sessionUsers;

  HostPairRequest._();

  factory HostPairRequest([void updates(HostPairRequestBuilder b)]) =
      _$HostPairRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostPairRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostPairRequest> get serializer =>
      _$HostPairRequestSerializer();
}

class _$HostPairRequestSerializer
    implements PrimitiveSerializer<HostPairRequest> {
  @override
  final Iterable<Type> types = const [HostPairRequest, _$HostPairRequest];

  @override
  final String wireName = r'HostPairRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostPairRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pairing_code';
    yield serializers.serialize(
      object.pairingCode,
      specifiedType: const FullType(String),
    );
    yield r'noise_public_key';
    yield serializers.serialize(
      object.noisePublicKey,
      specifiedType: const FullType(String),
    );
    yield r'auth_public_jwk';
    yield serializers.serialize(
      object.authPublicJwk,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(HostPlatform),
    );
    if (object.osVersion != null) {
      yield r'os_version';
      yield serializers.serialize(
        object.osVersion,
        specifiedType: const FullType(String),
      );
    }
    yield r'host_version';
    yield serializers.serialize(
      object.hostVersion,
      specifiedType: const FullType(String),
    );
    yield r'profiles';
    yield serializers.serialize(
      object.profiles,
      specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
    );
    yield r'session_users';
    yield serializers.serialize(
      object.sessionUsers,
      specifiedType: const FullType(BuiltList, [
        FullType(HostReportedSessionUser),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostPairRequest object, {
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
    required HostPairRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pairing_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pairingCode = valueDes;
          break;
        case r'noise_public_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.noisePublicKey = valueDes;
          break;
        case r'auth_public_jwk':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.authPublicJwk.replace(valueDes);
          break;
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostPlatform),
          ) as HostPlatform;
          result.platform = valueDes;
          break;
        case r'os_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.osVersion = valueDes;
          break;
        case r'host_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostVersion = valueDes;
          break;
        case r'profiles':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
          ) as BuiltList<HostProfile>;
          result.profiles.replace(valueDes);
          break;
        case r'session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(HostReportedSessionUser),
            ]),
          ) as BuiltList<HostReportedSessionUser>;
          result.sessionUsers.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostPairRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostPairRequestBuilder();
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
