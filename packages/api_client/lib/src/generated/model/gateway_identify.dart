//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/gateway_client_info.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_identify.g.dart';

/// People send `access_token`, `dpop`, `device_id`; agents and apps send `api_token`, `consumer` and `dpop` when the token is DPoP-bound.
///
/// Properties:
/// * [protocolVersion]
/// * [accessToken]
/// * [apiToken]
/// * [dpop] - DPoP proof with `htm = GET`, `htu` = HTTPS form of the gateway URL, `nonce` from `hello`, `ath`.
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [consumer]
/// * [cursor] - Last processed `seq`; absent on first connect.
/// * [syncEpoch] - Epoch stored with the cursor; absent on first connect. A mismatch gives `resync_required` with `epoch_changed`.
/// * [client]
@BuiltValue()
abstract class GatewayIdentify
    implements Built<GatewayIdentify, GatewayIdentifyBuilder> {
  @BuiltValueField(wireName: r'protocol_version')
  int get protocolVersion;

  @BuiltValueField(wireName: r'access_token')
  String? get accessToken;

  @BuiltValueField(wireName: r'api_token')
  String? get apiToken;

  /// DPoP proof with `htm = GET`, `htu` = HTTPS form of the gateway URL, `nonce` from `hello`, `ath`.
  @BuiltValueField(wireName: r'dpop')
  String? get dpop;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String? get deviceId;

  @BuiltValueField(wireName: r'consumer')
  String? get consumer;

  /// Last processed `seq`; absent on first connect.
  @BuiltValueField(wireName: r'cursor')
  int? get cursor;

  /// Epoch stored with the cursor; absent on first connect. A mismatch gives `resync_required` with `epoch_changed`.
  @BuiltValueField(wireName: r'sync_epoch')
  String? get syncEpoch;

  @BuiltValueField(wireName: r'client')
  GatewayClientInfo? get client;

  GatewayIdentify._();

  factory GatewayIdentify([void updates(GatewayIdentifyBuilder b)]) =
      _$GatewayIdentify;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayIdentifyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayIdentify> get serializer =>
      _$GatewayIdentifySerializer();
}

class _$GatewayIdentifySerializer
    implements PrimitiveSerializer<GatewayIdentify> {
  @override
  final Iterable<Type> types = const [GatewayIdentify, _$GatewayIdentify];

  @override
  final String wireName = r'GatewayIdentify';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayIdentify object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_version';
    yield serializers.serialize(
      object.protocolVersion,
      specifiedType: const FullType(int),
    );
    if (object.accessToken != null) {
      yield r'access_token';
      yield serializers.serialize(
        object.accessToken,
        specifiedType: const FullType(String),
      );
    }
    if (object.apiToken != null) {
      yield r'api_token';
      yield serializers.serialize(
        object.apiToken,
        specifiedType: const FullType(String),
      );
    }
    if (object.dpop != null) {
      yield r'dpop';
      yield serializers.serialize(
        object.dpop,
        specifiedType: const FullType(String),
      );
    }
    if (object.deviceId != null) {
      yield r'device_id';
      yield serializers.serialize(
        object.deviceId,
        specifiedType: const FullType(String),
      );
    }
    if (object.consumer != null) {
      yield r'consumer';
      yield serializers.serialize(
        object.consumer,
        specifiedType: const FullType(String),
      );
    }
    if (object.cursor != null) {
      yield r'cursor';
      yield serializers.serialize(
        object.cursor,
        specifiedType: const FullType(int),
      );
    }
    if (object.syncEpoch != null) {
      yield r'sync_epoch';
      yield serializers.serialize(
        object.syncEpoch,
        specifiedType: const FullType(String),
      );
    }
    if (object.client != null) {
      yield r'client';
      yield serializers.serialize(
        object.client,
        specifiedType: const FullType(GatewayClientInfo),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayIdentify object, {
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
    required GatewayIdentifyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'protocol_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.protocolVersion = valueDes;
          break;
        case r'access_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.accessToken = valueDes;
          break;
        case r'api_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.apiToken = valueDes;
          break;
        case r'dpop':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dpop = valueDes;
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.deviceId = valueDes;
          break;
        case r'consumer':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.consumer = valueDes;
          break;
        case r'cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.cursor = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.syncEpoch = valueDes;
          break;
        case r'client':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(GatewayClientInfo),
          ) as GatewayClientInfo?;
          if (valueDes == null) continue;
          result.client.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayIdentify deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayIdentifyBuilder();
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
