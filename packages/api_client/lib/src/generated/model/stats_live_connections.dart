//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_connections.g.dart';

/// Gateway connections of this node; only with `cluster.mode = single`.
///
/// Properties:
/// * [user]
/// * [agent]
/// * [app]
@BuiltValue()
abstract class StatsLiveConnections
    implements Built<StatsLiveConnections, StatsLiveConnectionsBuilder> {
  @BuiltValueField(wireName: r'user')
  int get user;

  @BuiltValueField(wireName: r'agent')
  int get agent;

  @BuiltValueField(wireName: r'app')
  int get app;

  StatsLiveConnections._();

  factory StatsLiveConnections([void updates(StatsLiveConnectionsBuilder b)]) =
      _$StatsLiveConnections;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveConnectionsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveConnections> get serializer =>
      _$StatsLiveConnectionsSerializer();
}

class _$StatsLiveConnectionsSerializer
    implements PrimitiveSerializer<StatsLiveConnections> {
  @override
  final Iterable<Type> types = const [
    StatsLiveConnections,
    _$StatsLiveConnections,
  ];

  @override
  final String wireName = r'StatsLiveConnections';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveConnections object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user';
    yield serializers.serialize(
      object.user,
      specifiedType: const FullType(int),
    );
    yield r'agent';
    yield serializers.serialize(
      object.agent,
      specifiedType: const FullType(int),
    );
    yield r'app';
    yield serializers.serialize(object.app, specifiedType: const FullType(int));
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveConnections object, {
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
    required StatsLiveConnectionsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.user = valueDes;
          break;
        case r'agent':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.agent = valueDes;
          break;
        case r'app':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.app = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveConnections deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveConnectionsBuilder();
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
