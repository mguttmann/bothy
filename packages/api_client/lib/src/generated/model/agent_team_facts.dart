//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_team_member.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_team_facts.g.dart';

/// The team facts of a `team` run (09 3.9.9). The owner's authority never comes from here; it comes only from the input message (origin `human` and an owner as author).
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [memberId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [handle]
/// * [source_]
/// * [sourceHandle] - The handle of the source member; set exactly for `source = member`.
/// * [addressed] - The input mentions the receiving member's handle.
/// * [members]
@BuiltValue()
abstract class AgentTeamFacts
    implements Built<AgentTeamFacts, AgentTeamFactsBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'member_id')
  String get memberId;

  @BuiltValueField(wireName: r'handle')
  String get handle;

  @BuiltValueField(wireName: r'source')
  AgentTeamFactsSource_Enum get source_;
  // enum source_Enum {  owner,  member,  };

  /// The handle of the source member; set exactly for `source = member`.
  @BuiltValueField(wireName: r'source_handle')
  String? get sourceHandle;

  /// The input mentions the receiving member's handle.
  @BuiltValueField(wireName: r'addressed')
  bool get addressed;

  @BuiltValueField(wireName: r'members')
  BuiltList<AgentTeamMember> get members;

  AgentTeamFacts._();

  factory AgentTeamFacts([void updates(AgentTeamFactsBuilder b)]) =
      _$AgentTeamFacts;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentTeamFactsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentTeamFacts> get serializer =>
      _$AgentTeamFactsSerializer();
}

class _$AgentTeamFactsSerializer
    implements PrimitiveSerializer<AgentTeamFacts> {
  @override
  final Iterable<Type> types = const [AgentTeamFacts, _$AgentTeamFacts];

  @override
  final String wireName = r'AgentTeamFacts';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentTeamFacts object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
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
    yield r'source';
    yield serializers.serialize(
      object.source_,
      specifiedType: const FullType(AgentTeamFactsSource_Enum),
    );
    if (object.sourceHandle != null) {
      yield r'source_handle';
      yield serializers.serialize(
        object.sourceHandle,
        specifiedType: const FullType(String),
      );
    }
    yield r'addressed';
    yield serializers.serialize(
      object.addressed,
      specifiedType: const FullType(bool),
    );
    yield r'members';
    yield serializers.serialize(
      object.members,
      specifiedType: const FullType(BuiltList, [FullType(AgentTeamMember)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentTeamFacts object, {
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
    required AgentTeamFactsBuilder result,
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
        case r'source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentTeamFactsSource_Enum),
          ) as AgentTeamFactsSource_Enum;
          result.source_ = valueDes;
          break;
        case r'source_handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sourceHandle = valueDes;
          break;
        case r'addressed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.addressed = valueDes;
          break;
        case r'members':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentTeamMember),
            ]),
          ) as BuiltList<AgentTeamMember>;
          result.members.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentTeamFacts deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentTeamFactsBuilder();
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

class AgentTeamFactsSource_Enum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'owner')
  static const AgentTeamFactsSource_Enum owner =
      _$agentTeamFactsSourceEnum_owner;
  @BuiltValueEnumConst(wireName: r'member')
  static const AgentTeamFactsSource_Enum member =
      _$agentTeamFactsSourceEnum_member;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentTeamFactsSource_Enum unknownDefaultOpenApi =
      _$agentTeamFactsSourceEnum_unknownDefaultOpenApi;

  static Serializer<AgentTeamFactsSource_Enum> get serializer =>
      _$agentTeamFactsSourceEnumSerializer;

  const AgentTeamFactsSource_Enum._(String name) : super(name);

  static BuiltSet<AgentTeamFactsSource_Enum> get values =>
      _$agentTeamFactsSourceEnumValues;
  static AgentTeamFactsSource_Enum valueOf(String name) =>
      _$agentTeamFactsSourceEnumValueOf(name);
}
