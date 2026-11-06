//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_interview_turn.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_request.g.dart';

/// The interview so far, kept by the client (26 6.1). Nothing of it is stored.
///
/// Properties:
/// * [turns] - The builder's earlier questions and the person's answers, oldest first; at most `agents.interview.max_turns`. Empty for the start of an interview.
/// * [wantDraft] - The person asks for a draft now instead of further questions.
@BuiltValue()
abstract class AgentInterviewRequest
    implements Built<AgentInterviewRequest, AgentInterviewRequestBuilder> {
  /// The builder's earlier questions and the person's answers, oldest first; at most `agents.interview.max_turns`. Empty for the start of an interview.
  @BuiltValueField(wireName: r'turns')
  BuiltList<AgentInterviewTurn> get turns;

  /// The person asks for a draft now instead of further questions.
  @BuiltValueField(wireName: r'want_draft')
  bool? get wantDraft;

  AgentInterviewRequest._();

  factory AgentInterviewRequest([
    void updates(AgentInterviewRequestBuilder b),
  ]) = _$AgentInterviewRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewRequest> get serializer =>
      _$AgentInterviewRequestSerializer();
}

class _$AgentInterviewRequestSerializer
    implements PrimitiveSerializer<AgentInterviewRequest> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewRequest,
    _$AgentInterviewRequest,
  ];

  @override
  final String wireName = r'AgentInterviewRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'turns';
    yield serializers.serialize(
      object.turns,
      specifiedType: const FullType(BuiltList, [FullType(AgentInterviewTurn)]),
    );
    if (object.wantDraft != null) {
      yield r'want_draft';
      yield serializers.serialize(
        object.wantDraft,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewRequest object, {
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
    required AgentInterviewRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'turns':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentInterviewTurn),
            ]),
          ) as BuiltList<AgentInterviewTurn>;
          result.turns.replace(valueDes);
          break;
        case r'want_draft':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.wantDraft = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewRequestBuilder();
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
