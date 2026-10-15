//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/public_branding.dart';
import 'package:api_client/src/generated/model/auth_provider_info.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/gateway_info.dart';
import 'package:api_client/src/generated/model/client_version_requirement.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'discovery.g.dart';

/// Public instance metadata for login screens, SDKs and adapters.
///
/// Properties:
/// * [instanceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [setupRequired] - True until the first owner exists (08, 4); every other operation answers `503 setup_required` until then.
/// * [apiVersions]
/// * [specVersion] - SemVer of openapi.yaml served by this instance.
/// * [features] - Feature keys this server implements beyond the base contract of its spec version.
/// * [gateway]
/// * [branding]
/// * [authProviders]
/// * [localPasswordEnabled]
/// * [jwksUri]
/// * [webhookKeysUri]
/// * [minClientVersions]
@BuiltValue()
abstract class Discovery implements Built<Discovery, DiscoveryBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'instance_id')
  String get instanceId;

  /// True until the first owner exists (08, 4); every other operation answers `503 setup_required` until then.
  @BuiltValueField(wireName: r'setup_required')
  bool get setupRequired;

  @BuiltValueField(wireName: r'api_versions')
  BuiltList<String> get apiVersions;

  /// SemVer of openapi.yaml served by this instance.
  @BuiltValueField(wireName: r'spec_version')
  String get specVersion;

  /// Feature keys this server implements beyond the base contract of its spec version.
  @BuiltValueField(wireName: r'features')
  BuiltList<String>? get features;

  @BuiltValueField(wireName: r'gateway')
  GatewayInfo get gateway;

  @BuiltValueField(wireName: r'branding')
  PublicBranding get branding;

  @BuiltValueField(wireName: r'auth_providers')
  BuiltList<AuthProviderInfo> get authProviders;

  @BuiltValueField(wireName: r'local_password_enabled')
  bool? get localPasswordEnabled;

  @BuiltValueField(wireName: r'jwks_uri')
  String get jwksUri;

  @BuiltValueField(wireName: r'webhook_keys_uri')
  String get webhookKeysUri;

  @BuiltValueField(wireName: r'min_client_versions')
  BuiltList<ClientVersionRequirement>? get minClientVersions;

  Discovery._();

  factory Discovery([void updates(DiscoveryBuilder b)]) = _$Discovery;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DiscoveryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Discovery> get serializer => _$DiscoverySerializer();
}

class _$DiscoverySerializer implements PrimitiveSerializer<Discovery> {
  @override
  final Iterable<Type> types = const [Discovery, _$Discovery];

  @override
  final String wireName = r'Discovery';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Discovery object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'instance_id';
    yield serializers.serialize(
      object.instanceId,
      specifiedType: const FullType(String),
    );
    yield r'setup_required';
    yield serializers.serialize(
      object.setupRequired,
      specifiedType: const FullType(bool),
    );
    yield r'api_versions';
    yield serializers.serialize(
      object.apiVersions,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'spec_version';
    yield serializers.serialize(
      object.specVersion,
      specifiedType: const FullType(String),
    );
    if (object.features != null) {
      yield r'features';
      yield serializers.serialize(
        object.features,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'gateway';
    yield serializers.serialize(
      object.gateway,
      specifiedType: const FullType(GatewayInfo),
    );
    yield r'branding';
    yield serializers.serialize(
      object.branding,
      specifiedType: const FullType(PublicBranding),
    );
    yield r'auth_providers';
    yield serializers.serialize(
      object.authProviders,
      specifiedType: const FullType(BuiltList, [FullType(AuthProviderInfo)]),
    );
    if (object.localPasswordEnabled != null) {
      yield r'local_password_enabled';
      yield serializers.serialize(
        object.localPasswordEnabled,
        specifiedType: const FullType(bool),
      );
    }
    yield r'jwks_uri';
    yield serializers.serialize(
      object.jwksUri,
      specifiedType: const FullType(String),
    );
    yield r'webhook_keys_uri';
    yield serializers.serialize(
      object.webhookKeysUri,
      specifiedType: const FullType(String),
    );
    if (object.minClientVersions != null) {
      yield r'min_client_versions';
      yield serializers.serialize(
        object.minClientVersions,
        specifiedType: const FullType(BuiltList, [
          FullType(ClientVersionRequirement),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Discovery object, {
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
    required DiscoveryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'instance_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.instanceId = valueDes;
          break;
        case r'setup_required':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.setupRequired = valueDes;
          break;
        case r'api_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.apiVersions.replace(valueDes);
          break;
        case r'spec_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.specVersion = valueDes;
          break;
        case r'features':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.features.replace(valueDes);
          break;
        case r'gateway':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GatewayInfo),
          ) as GatewayInfo;
          result.gateway.replace(valueDes);
          break;
        case r'branding':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PublicBranding),
          ) as PublicBranding;
          result.branding.replace(valueDes);
          break;
        case r'auth_providers':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AuthProviderInfo),
            ]),
          ) as BuiltList<AuthProviderInfo>;
          result.authProviders.replace(valueDes);
          break;
        case r'local_password_enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.localPasswordEnabled = valueDes;
          break;
        case r'jwks_uri':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.jwksUri = valueDes;
          break;
        case r'webhook_keys_uri':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.webhookKeysUri = valueDes;
          break;
        case r'min_client_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ClientVersionRequirement),
            ]),
          ) as BuiltList<ClientVersionRequirement>?;
          if (valueDes == null) continue;
          result.minClientVersions.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Discovery deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DiscoveryBuilder();
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
