//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/jwks.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_channel_hello.g.dart';

/// First text frame on the control and data connections of a host service. It carries server values the host service applies; the values of the `hello` of a data connection apply to the session of that connection (24, 5.3, 6.2 and 6.3).
///
/// Properties:
/// * [protocolVersions]
/// * [challenge] - Random value; the host signs it with its Ed25519 key in `identify`.
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [heartbeatIntervalMs]
/// * [heartbeatMissLimit]
/// * [clockLeewayMs] - `auth.clock_leeway` in milliseconds. The host service applies it to the session of this data connection: the tolerance on `exp` of the session token and how long it remembers the `jti` after `exp` (24, 5.3).
/// * [idleTimeoutMs] - `terminal.idle_timeout` in milliseconds. The host service applies it to the idle timer of the session of this data connection (24, 6.3).
/// * [detachGraceMs] - `terminal.detach_grace` in milliseconds. The host service applies it to the detach timer of the session of this data connection (24, 6.2).
/// * [jwks] - Public keys with purpose `terminal` for checking session tokens.
@BuiltValue()
abstract class HostChannelHello
    implements Built<HostChannelHello, HostChannelHelloBuilder> {
  @BuiltValueField(wireName: r'protocol_versions')
  BuiltList<int> get protocolVersions;

  /// Random value; the host signs it with its Ed25519 key in `identify`.
  @BuiltValueField(wireName: r'challenge')
  String get challenge;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime get serverTime;

  @BuiltValueField(wireName: r'heartbeat_interval_ms')
  int get heartbeatIntervalMs;

  @BuiltValueField(wireName: r'heartbeat_miss_limit')
  int get heartbeatMissLimit;

  /// `auth.clock_leeway` in milliseconds. The host service applies it to the session of this data connection: the tolerance on `exp` of the session token and how long it remembers the `jti` after `exp` (24, 5.3).
  @BuiltValueField(wireName: r'clock_leeway_ms')
  int get clockLeewayMs;

  /// `terminal.idle_timeout` in milliseconds. The host service applies it to the idle timer of the session of this data connection (24, 6.3).
  @BuiltValueField(wireName: r'idle_timeout_ms')
  int get idleTimeoutMs;

  /// `terminal.detach_grace` in milliseconds. The host service applies it to the detach timer of the session of this data connection (24, 6.2).
  @BuiltValueField(wireName: r'detach_grace_ms')
  int get detachGraceMs;

  /// Public keys with purpose `terminal` for checking session tokens.
  @BuiltValueField(wireName: r'jwks')
  Jwks get jwks;

  HostChannelHello._();

  factory HostChannelHello([void updates(HostChannelHelloBuilder b)]) =
      _$HostChannelHello;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostChannelHelloBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostChannelHello> get serializer =>
      _$HostChannelHelloSerializer();
}

class _$HostChannelHelloSerializer
    implements PrimitiveSerializer<HostChannelHello> {
  @override
  final Iterable<Type> types = const [HostChannelHello, _$HostChannelHello];

  @override
  final String wireName = r'HostChannelHello';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostChannelHello object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_versions';
    yield serializers.serialize(
      object.protocolVersions,
      specifiedType: const FullType(BuiltList, [FullType(int)]),
    );
    yield r'challenge';
    yield serializers.serialize(
      object.challenge,
      specifiedType: const FullType(String),
    );
    yield r'server_time';
    yield serializers.serialize(
      object.serverTime,
      specifiedType: const FullType(DateTime),
    );
    yield r'heartbeat_interval_ms';
    yield serializers.serialize(
      object.heartbeatIntervalMs,
      specifiedType: const FullType(int),
    );
    yield r'heartbeat_miss_limit';
    yield serializers.serialize(
      object.heartbeatMissLimit,
      specifiedType: const FullType(int),
    );
    yield r'clock_leeway_ms';
    yield serializers.serialize(
      object.clockLeewayMs,
      specifiedType: const FullType(int),
    );
    yield r'idle_timeout_ms';
    yield serializers.serialize(
      object.idleTimeoutMs,
      specifiedType: const FullType(int),
    );
    yield r'detach_grace_ms';
    yield serializers.serialize(
      object.detachGraceMs,
      specifiedType: const FullType(int),
    );
    yield r'jwks';
    yield serializers.serialize(
      object.jwks,
      specifiedType: const FullType(Jwks),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostChannelHello object, {
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
    required HostChannelHelloBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'protocol_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(int)]),
          ) as BuiltList<int>;
          result.protocolVersions.replace(valueDes);
          break;
        case r'challenge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.challenge = valueDes;
          break;
        case r'server_time':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.serverTime = valueDes;
          break;
        case r'heartbeat_interval_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.heartbeatIntervalMs = valueDes;
          break;
        case r'heartbeat_miss_limit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.heartbeatMissLimit = valueDes;
          break;
        case r'clock_leeway_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.clockLeewayMs = valueDes;
          break;
        case r'idle_timeout_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.idleTimeoutMs = valueDes;
          break;
        case r'detach_grace_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.detachGraceMs = valueDes;
          break;
        case r'jwks':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Jwks),
          ) as Jwks;
          result.jwks.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostChannelHello deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostChannelHelloBuilder();
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
