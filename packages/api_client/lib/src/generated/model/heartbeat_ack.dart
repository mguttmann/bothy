//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/project_trash_request.dart';
import 'package:api_client/src/generated/model/agent_runtime_status.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'heartbeat_ack.g.dart';

/// HeartbeatAck
///
/// Properties:
/// * [presence]
/// * [intervalMs] - Interval the adapter should keep, `sync.heartbeat_interval`.
/// * [projectTrash] - Runners only, the project folders of deleted forums this agent served whose deletion is final and whose move is not reported yet (09 18.15); absent when there are none.
@BuiltValue()
abstract class HeartbeatAck
    implements Built<HeartbeatAck, HeartbeatAckBuilder> {
  @BuiltValueField(wireName: r'presence')
  AgentRuntimeStatus get presence;

  /// Interval the adapter should keep, `sync.heartbeat_interval`.
  @BuiltValueField(wireName: r'interval_ms')
  int get intervalMs;

  /// Runners only, the project folders of deleted forums this agent served whose deletion is final and whose move is not reported yet (09 18.15); absent when there are none.
  @BuiltValueField(wireName: r'project_trash')
  BuiltList<ProjectTrashRequest>? get projectTrash;

  HeartbeatAck._();

  factory HeartbeatAck([void updates(HeartbeatAckBuilder b)]) = _$HeartbeatAck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HeartbeatAckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HeartbeatAck> get serializer => _$HeartbeatAckSerializer();
}

class _$HeartbeatAckSerializer implements PrimitiveSerializer<HeartbeatAck> {
  @override
  final Iterable<Type> types = const [HeartbeatAck, _$HeartbeatAck];

  @override
  final String wireName = r'HeartbeatAck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HeartbeatAck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'presence';
    yield serializers.serialize(
      object.presence,
      specifiedType: const FullType(AgentRuntimeStatus),
    );
    yield r'interval_ms';
    yield serializers.serialize(
      object.intervalMs,
      specifiedType: const FullType(int),
    );
    if (object.projectTrash != null) {
      yield r'project_trash';
      yield serializers.serialize(
        object.projectTrash,
        specifiedType: const FullType(BuiltList, [
          FullType(ProjectTrashRequest),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HeartbeatAck object, {
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
    required HeartbeatAckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'presence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentRuntimeStatus),
          ) as AgentRuntimeStatus;
          result.presence.replace(valueDes);
          break;
        case r'interval_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.intervalMs = valueDes;
          break;
        case r'project_trash':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ProjectTrashRequest),
            ]),
          ) as BuiltList<ProjectTrashRequest>?;
          if (valueDes == null) continue;
          result.projectTrash.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HeartbeatAck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HeartbeatAckBuilder();
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
