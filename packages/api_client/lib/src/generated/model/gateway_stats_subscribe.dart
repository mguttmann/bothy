//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_stats_subscribe.g.dart';

/// Subscribe to `stats.live`; without `space_id` the instance (owner). Replaces an earlier subscription of the connection.
///
/// Properties:
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class GatewayStatsSubscribe
    implements Built<GatewayStatsSubscribe, GatewayStatsSubscribeBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  GatewayStatsSubscribe._();

  factory GatewayStatsSubscribe([
    void updates(GatewayStatsSubscribeBuilder b),
  ]) = _$GatewayStatsSubscribe;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayStatsSubscribeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayStatsSubscribe> get serializer =>
      _$GatewayStatsSubscribeSerializer();
}

class _$GatewayStatsSubscribeSerializer
    implements PrimitiveSerializer<GatewayStatsSubscribe> {
  @override
  final Iterable<Type> types = const [
    GatewayStatsSubscribe,
    _$GatewayStatsSubscribe,
  ];

  @override
  final String wireName = r'GatewayStatsSubscribe';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayStatsSubscribe object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayStatsSubscribe object, {
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
    required GatewayStatsSubscribeBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayStatsSubscribe deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayStatsSubscribeBuilder();
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
