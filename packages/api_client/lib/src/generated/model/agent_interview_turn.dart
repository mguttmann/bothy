//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_turn.g.dart';

/// AgentInterviewTurn
///
/// Properties:
/// * [role] - `builder` for a question of the builder, `owner` for an answer of the person.
/// * [text] - At most `agents.interview.answer_max_chars` characters; line breaks allowed, no other control or bidi character.
@BuiltValue()
abstract class AgentInterviewTurn
    implements Built<AgentInterviewTurn, AgentInterviewTurnBuilder> {
  /// `builder` for a question of the builder, `owner` for an answer of the person.
  @BuiltValueField(wireName: r'role')
  AgentInterviewTurnRoleEnum get role;
  // enum roleEnum {  builder,  owner,  };

  /// At most `agents.interview.answer_max_chars` characters; line breaks allowed, no other control or bidi character.
  @BuiltValueField(wireName: r'text')
  String get text;

  AgentInterviewTurn._();

  factory AgentInterviewTurn([void updates(AgentInterviewTurnBuilder b)]) =
      _$AgentInterviewTurn;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewTurnBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewTurn> get serializer =>
      _$AgentInterviewTurnSerializer();
}

class _$AgentInterviewTurnSerializer
    implements PrimitiveSerializer<AgentInterviewTurn> {
  @override
  final Iterable<Type> types = const [AgentInterviewTurn, _$AgentInterviewTurn];

  @override
  final String wireName = r'AgentInterviewTurn';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewTurn object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'role';
    yield serializers.serialize(
      object.role,
      specifiedType: const FullType(AgentInterviewTurnRoleEnum),
    );
    yield r'text';
    yield serializers.serialize(
      object.text,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewTurn object, {
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
    required AgentInterviewTurnBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'role':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentInterviewTurnRoleEnum),
          ) as AgentInterviewTurnRoleEnum;
          result.role = valueDes;
          break;
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.text = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewTurn deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewTurnBuilder();
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

/// `builder` for a question of the builder, `owner` for an answer of the person.
class AgentInterviewTurnRoleEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'builder')
  static const AgentInterviewTurnRoleEnum builder =
      _$agentInterviewTurnRoleEnum_builder;
  @BuiltValueEnumConst(wireName: r'owner')
  static const AgentInterviewTurnRoleEnum owner =
      _$agentInterviewTurnRoleEnum_owner;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInterviewTurnRoleEnum unknownDefaultOpenApi =
      _$agentInterviewTurnRoleEnum_unknownDefaultOpenApi;

  static Serializer<AgentInterviewTurnRoleEnum> get serializer =>
      _$agentInterviewTurnRoleEnumSerializer;

  const AgentInterviewTurnRoleEnum._(String name) : super(name);

  static BuiltSet<AgentInterviewTurnRoleEnum> get values =>
      _$agentInterviewTurnRoleEnumValues;
  static AgentInterviewTurnRoleEnum valueOf(String name) =>
      _$agentInterviewTurnRoleEnumValueOf(name);
}
