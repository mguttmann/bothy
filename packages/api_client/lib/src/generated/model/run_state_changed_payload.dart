//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/trigger_state.dart';
import 'package:api_client/src/generated/model/run_summary.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_state_changed_payload.g.dart';

/// RunStateChangedPayload
///
/// Properties:
/// * [run]
/// * [triggeringMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerState]
@BuiltValue()
abstract class RunStateChangedPayload
    implements Built<RunStateChangedPayload, RunStateChangedPayloadBuilder> {
  @BuiltValueField(wireName: r'run')
  RunSummary get run;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggering_message_id')
  String? get triggeringMessageId;

  @BuiltValueField(wireName: r'trigger_state')
  TriggerState? get triggerState;
  // enum triggerStateEnum {  received,  queued,  working,  done,  failed,  cancelled,  followup_delivered,  };

  RunStateChangedPayload._();

  factory RunStateChangedPayload([
    void updates(RunStateChangedPayloadBuilder b),
  ]) = _$RunStateChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunStateChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunStateChangedPayload> get serializer =>
      _$RunStateChangedPayloadSerializer();
}

class _$RunStateChangedPayloadSerializer
    implements PrimitiveSerializer<RunStateChangedPayload> {
  @override
  final Iterable<Type> types = const [
    RunStateChangedPayload,
    _$RunStateChangedPayload,
  ];

  @override
  final String wireName = r'RunStateChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunStateChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run';
    yield serializers.serialize(
      object.run,
      specifiedType: const FullType(RunSummary),
    );
    if (object.triggeringMessageId != null) {
      yield r'triggering_message_id';
      yield serializers.serialize(
        object.triggeringMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggerState != null) {
      yield r'trigger_state';
      yield serializers.serialize(
        object.triggerState,
        specifiedType: const FullType(TriggerState),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunStateChangedPayload object, {
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
    required RunStateChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunSummary),
          ) as RunSummary;
          result.run.replace(valueDes);
          break;
        case r'triggering_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.triggeringMessageId = valueDes;
          break;
        case r'trigger_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TriggerState),
          ) as TriggerState?;
          if (valueDes == null) continue;
          result.triggerState = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunStateChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunStateChangedPayloadBuilder();
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
