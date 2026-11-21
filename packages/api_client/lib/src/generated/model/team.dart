//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/team_pause_reason.dart';
import 'package:api_client/src/generated/model/team_member.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'team.g.dart';

/// The team of a team channel (09 3.9.1); `pause_reason` is absent while member inputs flow.
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [sponsorId] - The active owner on whose behalf member inputs run (09 3.9.2).
/// * [memberTurns] - Member inputs since an owner's last message (09 3.9.6).
/// * [turnBudget] - The value of `team.turn_budget` now.
/// * [pauseReason]
/// * [members]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
@BuiltValue()
abstract class Team implements Built<Team, TeamBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// The active owner on whose behalf member inputs run (09 3.9.2).
  @BuiltValueField(wireName: r'sponsor_id')
  String get sponsorId;

  /// Member inputs since an owner's last message (09 3.9.6).
  @BuiltValueField(wireName: r'member_turns')
  int get memberTurns;

  /// The value of `team.turn_budget` now.
  @BuiltValueField(wireName: r'turn_budget')
  int get turnBudget;

  @BuiltValueField(wireName: r'pause_reason')
  TeamPauseReason? get pauseReason;
  // enum pauseReasonEnum {  turn_budget,  pingpong,  stopped,  };

  @BuiltValueField(wireName: r'members')
  BuiltList<TeamMember> get members;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  Team._();

  factory Team([void updates(TeamBuilder b)]) = _$Team;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TeamBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Team> get serializer => _$TeamSerializer();
}

class _$TeamSerializer implements PrimitiveSerializer<Team> {
  @override
  final Iterable<Type> types = const [Team, _$Team];

  @override
  final String wireName = r'Team';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Team object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'sponsor_id';
    yield serializers.serialize(
      object.sponsorId,
      specifiedType: const FullType(String),
    );
    yield r'member_turns';
    yield serializers.serialize(
      object.memberTurns,
      specifiedType: const FullType(int),
    );
    yield r'turn_budget';
    yield serializers.serialize(
      object.turnBudget,
      specifiedType: const FullType(int),
    );
    if (object.pauseReason != null) {
      yield r'pause_reason';
      yield serializers.serialize(
        object.pauseReason,
        specifiedType: const FullType(TeamPauseReason),
      );
    }
    yield r'members';
    yield serializers.serialize(
      object.members,
      specifiedType: const FullType(BuiltList, [FullType(TeamMember)]),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Team object, {
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
    required TeamBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'sponsor_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sponsorId = valueDes;
          break;
        case r'member_turns':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.memberTurns = valueDes;
          break;
        case r'turn_budget':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.turnBudget = valueDes;
          break;
        case r'pause_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TeamPauseReason),
          ) as TeamPauseReason?;
          if (valueDes == null) continue;
          result.pauseReason = valueDes;
          break;
        case r'members':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(TeamMember)]),
          ) as BuiltList<TeamMember>;
          result.members.replace(valueDes);
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Team deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TeamBuilder();
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
