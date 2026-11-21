//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_team_member.g.dart';

/// AgentTeamMember
///
/// Properties:
/// * [memberId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [handle]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [homeChannelId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class AgentTeamMember
    implements Built<AgentTeamMember, AgentTeamMemberBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'member_id')
  String get memberId;

  @BuiltValueField(wireName: r'handle')
  String get handle;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'home_channel_id')
  String get homeChannelId;

  AgentTeamMember._();

  factory AgentTeamMember([void updates(AgentTeamMemberBuilder b)]) =
      _$AgentTeamMember;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentTeamMemberBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentTeamMember> get serializer =>
      _$AgentTeamMemberSerializer();
}

class _$AgentTeamMemberSerializer
    implements PrimitiveSerializer<AgentTeamMember> {
  @override
  final Iterable<Type> types = const [AgentTeamMember, _$AgentTeamMember];

  @override
  final String wireName = r'AgentTeamMember';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentTeamMember object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'member_id';
    yield serializers.serialize(
      object.memberId,
      specifiedType: const FullType(String),
    );
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'home_channel_id';
    yield serializers.serialize(
      object.homeChannelId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentTeamMember object, {
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
    required AgentTeamMemberBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'member_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.memberId = valueDes;
          break;
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'home_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.homeChannelId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentTeamMember deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentTeamMemberBuilder();
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
