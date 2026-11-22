//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'team_member_create.g.dart';

/// TeamMemberCreate
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [homeChannelId] - A channel the agent operates (its effective operator); a project forum for a runner.
/// * [handle] - Unique in the team channel, at most `team.handle_max_chars` characters.
@BuiltValue()
abstract class TeamMemberCreate
    implements Built<TeamMemberCreate, TeamMemberCreateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// A channel the agent operates (its effective operator); a project forum for a runner.
  @BuiltValueField(wireName: r'home_channel_id')
  String get homeChannelId;

  /// Unique in the team channel, at most `team.handle_max_chars` characters.
  @BuiltValueField(wireName: r'handle')
  String get handle;

  TeamMemberCreate._();

  factory TeamMemberCreate([void updates(TeamMemberCreateBuilder b)]) =
      _$TeamMemberCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TeamMemberCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TeamMemberCreate> get serializer =>
      _$TeamMemberCreateSerializer();
}

class _$TeamMemberCreateSerializer
    implements PrimitiveSerializer<TeamMemberCreate> {
  @override
  final Iterable<Type> types = const [TeamMemberCreate, _$TeamMemberCreate];

  @override
  final String wireName = r'TeamMemberCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TeamMemberCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TeamMemberCreate object, {
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
    required TeamMemberCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TeamMemberCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TeamMemberCreateBuilder();
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
