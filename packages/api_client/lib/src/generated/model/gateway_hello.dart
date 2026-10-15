//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_hello.g.dart';

/// GatewayHello
///
/// Properties:
/// * [heartbeatIntervalMs]
/// * [resumeWindowMs]
/// * [protocolVersions]
/// * [dpopNonce]
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [syncEpoch] - Time line of the event log; a client stores it with its cursor (04, 7.5).
@BuiltValue()
abstract class GatewayHello
    implements Built<GatewayHello, GatewayHelloBuilder> {
  @BuiltValueField(wireName: r'heartbeat_interval_ms')
  int get heartbeatIntervalMs;

  @BuiltValueField(wireName: r'resume_window_ms')
  int get resumeWindowMs;

  @BuiltValueField(wireName: r'protocol_versions')
  BuiltList<int> get protocolVersions;

  @BuiltValueField(wireName: r'dpop_nonce')
  String get dpopNonce;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime get serverTime;

  /// Time line of the event log; a client stores it with its cursor (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  GatewayHello._();

  factory GatewayHello([void updates(GatewayHelloBuilder b)]) = _$GatewayHello;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayHelloBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayHello> get serializer => _$GatewayHelloSerializer();
}

class _$GatewayHelloSerializer implements PrimitiveSerializer<GatewayHello> {
  @override
  final Iterable<Type> types = const [GatewayHello, _$GatewayHello];

  @override
  final String wireName = r'GatewayHello';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayHello object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'heartbeat_interval_ms';
    yield serializers.serialize(
      object.heartbeatIntervalMs,
      specifiedType: const FullType(int),
    );
    yield r'resume_window_ms';
    yield serializers.serialize(
      object.resumeWindowMs,
      specifiedType: const FullType(int),
    );
    yield r'protocol_versions';
    yield serializers.serialize(
      object.protocolVersions,
      specifiedType: const FullType(BuiltList, [FullType(int)]),
    );
    yield r'dpop_nonce';
    yield serializers.serialize(
      object.dpopNonce,
      specifiedType: const FullType(String),
    );
    yield r'server_time';
    yield serializers.serialize(
      object.serverTime,
      specifiedType: const FullType(DateTime),
    );
    yield r'sync_epoch';
    yield serializers.serialize(
      object.syncEpoch,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayHello object, {
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
    required GatewayHelloBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'heartbeat_interval_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.heartbeatIntervalMs = valueDes;
          break;
        case r'resume_window_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.resumeWindowMs = valueDes;
          break;
        case r'protocol_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(int)]),
          ) as BuiltList<int>;
          result.protocolVersions.replace(valueDes);
          break;
        case r'dpop_nonce':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dpopNonce = valueDes;
          break;
        case r'server_time':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.serverTime = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.syncEpoch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayHello deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayHelloBuilder();
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
