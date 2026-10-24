//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_followup_payload.g.dart';

/// AgentFollowupPayload
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [messageId] - Follow-up message; the adapter reads its content with `getMessage` (`message.read`); its author is `sub` of the invocation token of this delivery.
@BuiltValue()
abstract class AgentFollowupPayload
    implements Built<AgentFollowupPayload, AgentFollowupPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// Follow-up message; the adapter reads its content with `getMessage` (`message.read`); its author is `sub` of the invocation token of this delivery.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  AgentFollowupPayload._();

  factory AgentFollowupPayload([void updates(AgentFollowupPayloadBuilder b)]) =
      _$AgentFollowupPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentFollowupPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentFollowupPayload> get serializer =>
      _$AgentFollowupPayloadSerializer();
}

class _$AgentFollowupPayloadSerializer
    implements PrimitiveSerializer<AgentFollowupPayload> {
  @override
  final Iterable<Type> types = const [
    AgentFollowupPayload,
    _$AgentFollowupPayload,
  ];

  @override
  final String wireName = r'AgentFollowupPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentFollowupPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentFollowupPayload object, {
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
    required AgentFollowupPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentFollowupPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentFollowupPayloadBuilder();
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
