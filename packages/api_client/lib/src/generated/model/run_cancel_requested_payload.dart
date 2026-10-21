//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_cancel_requested_payload.g.dart';

/// RunCancelRequestedPayload
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
/// * [requestedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class RunCancelRequestedPayload
    implements
        Built<RunCancelRequestedPayload, RunCancelRequestedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  @BuiltValueField(wireName: r'reason')
  RunCancelRequestedPayloadReasonEnum get reason;
  // enum reasonEnum {  user_cancelled,  access_revoked,  agent_disabled,  user_disabled,  channel_read_only,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'requested_by')
  String? get requestedBy;

  RunCancelRequestedPayload._();

  factory RunCancelRequestedPayload([
    void updates(RunCancelRequestedPayloadBuilder b),
  ]) = _$RunCancelRequestedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunCancelRequestedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunCancelRequestedPayload> get serializer =>
      _$RunCancelRequestedPayloadSerializer();
}

class _$RunCancelRequestedPayloadSerializer
    implements PrimitiveSerializer<RunCancelRequestedPayload> {
  @override
  final Iterable<Type> types = const [
    RunCancelRequestedPayload,
    _$RunCancelRequestedPayload,
  ];

  @override
  final String wireName = r'RunCancelRequestedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunCancelRequestedPayload object, {
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
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(RunCancelRequestedPayloadReasonEnum),
    );
    if (object.requestedBy != null) {
      yield r'requested_by';
      yield serializers.serialize(
        object.requestedBy,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunCancelRequestedPayload object, {
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
    required RunCancelRequestedPayloadBuilder result,
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
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunCancelRequestedPayloadReasonEnum),
          ) as RunCancelRequestedPayloadReasonEnum;
          result.reason = valueDes;
          break;
        case r'requested_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.requestedBy = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunCancelRequestedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunCancelRequestedPayloadBuilder();
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

class RunCancelRequestedPayloadReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user_cancelled')
  static const RunCancelRequestedPayloadReasonEnum userCancelled =
      _$runCancelRequestedPayloadReasonEnum_userCancelled;
  @BuiltValueEnumConst(wireName: r'access_revoked')
  static const RunCancelRequestedPayloadReasonEnum accessRevoked =
      _$runCancelRequestedPayloadReasonEnum_accessRevoked;
  @BuiltValueEnumConst(wireName: r'agent_disabled')
  static const RunCancelRequestedPayloadReasonEnum agentDisabled =
      _$runCancelRequestedPayloadReasonEnum_agentDisabled;
  @BuiltValueEnumConst(wireName: r'user_disabled')
  static const RunCancelRequestedPayloadReasonEnum userDisabled =
      _$runCancelRequestedPayloadReasonEnum_userDisabled;
  @BuiltValueEnumConst(wireName: r'channel_read_only')
  static const RunCancelRequestedPayloadReasonEnum channelReadOnly =
      _$runCancelRequestedPayloadReasonEnum_channelReadOnly;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunCancelRequestedPayloadReasonEnum unknownDefaultOpenApi =
      _$runCancelRequestedPayloadReasonEnum_unknownDefaultOpenApi;

  static Serializer<RunCancelRequestedPayloadReasonEnum> get serializer =>
      _$runCancelRequestedPayloadReasonEnumSerializer;

  const RunCancelRequestedPayloadReasonEnum._(String name) : super(name);

  static BuiltSet<RunCancelRequestedPayloadReasonEnum> get values =>
      _$runCancelRequestedPayloadReasonEnumValues;
  static RunCancelRequestedPayloadReasonEnum valueOf(String name) =>
      _$runCancelRequestedPayloadReasonEnumValueOf(name);
}
