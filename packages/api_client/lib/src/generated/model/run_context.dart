//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_context_message.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_context.g.dart';

/// Context of an own active run, schema `run_context.v1` (09, 4.6). Built by the server because only `authz` knows how trustworthy each author is.
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerMessage]
/// * [threadContext]
/// * [channelContext] - Only on the first run of a session in a thread with `parent_message_id`.
/// * [contextFloorSeq]
/// * [truncatedBeforeSeq] - Set when older messages are missing; load them with `thread.read`.
@BuiltValue()
abstract class RunContext implements Built<RunContext, RunContextBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'trigger_message')
  RunContextMessage get triggerMessage;

  @BuiltValueField(wireName: r'thread_context')
  BuiltList<RunContextMessage> get threadContext;

  /// Only on the first run of a session in a thread with `parent_message_id`.
  @BuiltValueField(wireName: r'channel_context')
  BuiltList<RunContextMessage>? get channelContext;

  @BuiltValueField(wireName: r'context_floor_seq')
  int get contextFloorSeq;

  /// Set when older messages are missing; load them with `thread.read`.
  @BuiltValueField(wireName: r'truncated_before_seq')
  int? get truncatedBeforeSeq;

  RunContext._();

  factory RunContext([void updates(RunContextBuilder b)]) = _$RunContext;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunContextBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunContext> get serializer => _$RunContextSerializer();
}

class _$RunContextSerializer implements PrimitiveSerializer<RunContext> {
  @override
  final Iterable<Type> types = const [RunContext, _$RunContext];

  @override
  final String wireName = r'RunContext';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunContext object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'trigger_message';
    yield serializers.serialize(
      object.triggerMessage,
      specifiedType: const FullType(RunContextMessage),
    );
    yield r'thread_context';
    yield serializers.serialize(
      object.threadContext,
      specifiedType: const FullType(BuiltList, [FullType(RunContextMessage)]),
    );
    if (object.channelContext != null) {
      yield r'channel_context';
      yield serializers.serialize(
        object.channelContext,
        specifiedType: const FullType(BuiltList, [FullType(RunContextMessage)]),
      );
    }
    yield r'context_floor_seq';
    yield serializers.serialize(
      object.contextFloorSeq,
      specifiedType: const FullType(int),
    );
    if (object.truncatedBeforeSeq != null) {
      yield r'truncated_before_seq';
      yield serializers.serialize(
        object.truncatedBeforeSeq,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunContext object, {
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
    required RunContextBuilder result,
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
        case r'trigger_message':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunContextMessage),
          ) as RunContextMessage;
          result.triggerMessage.replace(valueDes);
          break;
        case r'thread_context':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(RunContextMessage),
            ]),
          ) as BuiltList<RunContextMessage>;
          result.threadContext.replace(valueDes);
          break;
        case r'channel_context':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(RunContextMessage),
            ]),
          ) as BuiltList<RunContextMessage>?;
          if (valueDes == null) continue;
          result.channelContext.replace(valueDes);
          break;
        case r'context_floor_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.contextFloorSeq = valueDes;
          break;
        case r'truncated_before_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.truncatedBeforeSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunContext deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunContextBuilder();
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
