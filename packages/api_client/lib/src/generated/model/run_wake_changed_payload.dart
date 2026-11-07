//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_wake_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_changed_payload.g.dart';

/// Payload of `run.wake_changed` (09 5.8.2); ids and codes only, never the label, the condition or the result. Clients reload the thread's wake-ups with `listThreadWakes`, a runner with `listPendingRunWakes`.
///
/// Properties:
/// * [wakeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggeredBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [stateReason] - Reason code of the change.
/// * [noteMessageId] - The note in the thread of an end other than `fired` (09 5.8.4).
/// * [noteThreadSeq] - The `thread_seq` of the note; set with `note_message_id`.
@BuiltValue()
abstract class RunWakeChangedPayload
    implements Built<RunWakeChangedPayload, RunWakeChangedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'wake_id')
  String get wakeId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggered_by')
  String get triggeredBy;

  @BuiltValueField(wireName: r'state')
  RunWakeState get state;
  // enum stateEnum {  pending,  fired,  cancelled,  expired,  refused,  failed,  };

  /// Reason code of the change.
  @BuiltValueField(wireName: r'state_reason')
  String? get stateReason;

  /// The note in the thread of an end other than `fired` (09 5.8.4).
  @BuiltValueField(wireName: r'note_message_id')
  String? get noteMessageId;

  /// The `thread_seq` of the note; set with `note_message_id`.
  @BuiltValueField(wireName: r'note_thread_seq')
  int? get noteThreadSeq;

  RunWakeChangedPayload._();

  factory RunWakeChangedPayload([
    void updates(RunWakeChangedPayloadBuilder b),
  ]) = _$RunWakeChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeChangedPayload> get serializer =>
      _$RunWakeChangedPayloadSerializer();
}

class _$RunWakeChangedPayloadSerializer
    implements PrimitiveSerializer<RunWakeChangedPayload> {
  @override
  final Iterable<Type> types = const [
    RunWakeChangedPayload,
    _$RunWakeChangedPayload,
  ];

  @override
  final String wireName = r'RunWakeChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'wake_id';
    yield serializers.serialize(
      object.wakeId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'triggered_by';
    yield serializers.serialize(
      object.triggeredBy,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(RunWakeState),
    );
    if (object.stateReason != null) {
      yield r'state_reason';
      yield serializers.serialize(
        object.stateReason,
        specifiedType: const FullType(String),
      );
    }
    if (object.noteMessageId != null) {
      yield r'note_message_id';
      yield serializers.serialize(
        object.noteMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.noteThreadSeq != null) {
      yield r'note_thread_seq';
      yield serializers.serialize(
        object.noteThreadSeq,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeChangedPayload object, {
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
    required RunWakeChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'wake_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.wakeId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'triggered_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.triggeredBy = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeState),
          ) as RunWakeState;
          result.state = valueDes;
          break;
        case r'state_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.stateReason = valueDes;
          break;
        case r'note_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.noteMessageId = valueDes;
          break;
        case r'note_thread_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.noteThreadSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeChangedPayloadBuilder();
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
