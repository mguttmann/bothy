//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/permission_hint_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/grant.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'permission_hint.g.dart';

/// One permission hint. `h1_read_horizon` names `agent_id`, `person_id`, `space_id` and `channel_id` (the person can learn content of the channel through the agent, 07 section 6). `grant_to_disabled` and `trigger_blocked` carry the `grant`; `trigger_blocked` names the blocking `rules` (SR2 tool class, SR3 dev isolation). `agent_personal` names `agent_id`, `credential_holder_id` (absent when the agent has none) and the `trigger_set` (I1 asks for exactly the holder).
///
/// Properties:
/// * [kind]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [personId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [grant]
/// * [rules]
/// * [credentialHolderId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerSet] - The people with effective `trigger` on a target the agent serves, ascending.
@BuiltValue()
abstract class PermissionHint
    implements Built<PermissionHint, PermissionHintBuilder> {
  @BuiltValueField(wireName: r'kind')
  PermissionHintKind get kind;
  // enum kindEnum {  h1_read_horizon,  grant_to_disabled,  trigger_blocked,  agent_personal,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'person_id')
  String? get personId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String? get channelId;

  @BuiltValueField(wireName: r'grant')
  Grant? get grant;

  @BuiltValueField(wireName: r'rules')
  BuiltList<PermissionHintRulesEnum>? get rules;
  // enum rulesEnum {  sr2,  sr3,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  /// The people with effective `trigger` on a target the agent serves, ascending.
  @BuiltValueField(wireName: r'trigger_set')
  BuiltList<String>? get triggerSet;

  PermissionHint._();

  factory PermissionHint([void updates(PermissionHintBuilder b)]) =
      _$PermissionHint;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PermissionHintBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PermissionHint> get serializer =>
      _$PermissionHintSerializer();
}

class _$PermissionHintSerializer
    implements PrimitiveSerializer<PermissionHint> {
  @override
  final Iterable<Type> types = const [PermissionHint, _$PermissionHint];

  @override
  final String wireName = r'PermissionHint';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PermissionHint object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(PermissionHintKind),
    );
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.personId != null) {
      yield r'person_id';
      yield serializers.serialize(
        object.personId,
        specifiedType: const FullType(String),
      );
    }
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
    if (object.channelId != null) {
      yield r'channel_id';
      yield serializers.serialize(
        object.channelId,
        specifiedType: const FullType(String),
      );
    }
    if (object.grant != null) {
      yield r'grant';
      yield serializers.serialize(
        object.grant,
        specifiedType: const FullType(Grant),
      );
    }
    if (object.rules != null) {
      yield r'rules';
      yield serializers.serialize(
        object.rules,
        specifiedType: const FullType(BuiltList, [
          FullType(PermissionHintRulesEnum),
        ]),
      );
    }
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggerSet != null) {
      yield r'trigger_set';
      yield serializers.serialize(
        object.triggerSet,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PermissionHint object, {
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
    required PermissionHintBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PermissionHintKind),
          ) as PermissionHintKind;
          result.kind = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'person_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.personId = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.channelId = valueDes;
          break;
        case r'grant':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Grant),
          ) as Grant?;
          if (valueDes == null) continue;
          result.grant.replace(valueDes);
          break;
        case r'rules':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(PermissionHintRulesEnum),
            ]),
          ) as BuiltList<PermissionHintRulesEnum>?;
          if (valueDes == null) continue;
          result.rules.replace(valueDes);
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.credentialHolderId = valueDes;
          break;
        case r'trigger_set':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.triggerSet.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PermissionHint deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PermissionHintBuilder();
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

class PermissionHintRulesEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'sr2')
  static const PermissionHintRulesEnum sr2 = _$permissionHintRulesEnum_sr2;
  @BuiltValueEnumConst(wireName: r'sr3')
  static const PermissionHintRulesEnum sr3 = _$permissionHintRulesEnum_sr3;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PermissionHintRulesEnum unknownDefaultOpenApi =
      _$permissionHintRulesEnum_unknownDefaultOpenApi;

  static Serializer<PermissionHintRulesEnum> get serializer =>
      _$permissionHintRulesEnumSerializer;

  const PermissionHintRulesEnum._(String name) : super(name);

  static BuiltSet<PermissionHintRulesEnum> get values =>
      _$permissionHintRulesEnumValues;
  static PermissionHintRulesEnum valueOf(String name) =>
      _$permissionHintRulesEnumValueOf(name);
}
