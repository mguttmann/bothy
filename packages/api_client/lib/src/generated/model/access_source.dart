//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_source.g.dart';

/// AccessSource
///
/// Properties:
/// * [kind]
/// * [grantId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [groupId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [fromTargetKind]
/// * [fromTargetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - Agent of an `operator`, `via_agent` or `guard_blocked` source.
/// * [rule] - Guard rule of a `guard_blocked` source.
/// * [expiresAt] - Expiry of the grant.
/// * [membershipExpiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [reason] - Reason of the grant.
@BuiltValue()
abstract class AccessSource
    implements Built<AccessSource, AccessSourceBuilder> {
  @BuiltValueField(wireName: r'kind')
  AccessSourceKindEnum get kind;
  // enum kindEnum {  owner,  direct,  group,  inherited,  dm_participant,  operator,  via_agent,  guard_blocked,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'grant_id')
  String? get grantId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'group_id')
  String? get groupId;

  @BuiltValueField(wireName: r'from_target_kind')
  TargetKind? get fromTargetKind;
  // enum fromTargetKindEnum {  instance,  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'from_target_id')
  String? get fromTargetId;

  /// Agent of an `operator`, `via_agent` or `guard_blocked` source.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// Guard rule of a `guard_blocked` source.
  @BuiltValueField(wireName: r'rule')
  AccessSourceRuleEnum? get rule;
  // enum ruleEnum {  sr2,  sr3,  };

  /// Expiry of the grant.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'membership_expires_at')
  DateTime? get membershipExpiresAt;

  /// Reason of the grant.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  AccessSource._();

  factory AccessSource([void updates(AccessSourceBuilder b)]) = _$AccessSource;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessSourceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessSource> get serializer => _$AccessSourceSerializer();
}

class _$AccessSourceSerializer implements PrimitiveSerializer<AccessSource> {
  @override
  final Iterable<Type> types = const [AccessSource, _$AccessSource];

  @override
  final String wireName = r'AccessSource';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessSource object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(AccessSourceKindEnum),
    );
    if (object.grantId != null) {
      yield r'grant_id';
      yield serializers.serialize(
        object.grantId,
        specifiedType: const FullType(String),
      );
    }
    if (object.groupId != null) {
      yield r'group_id';
      yield serializers.serialize(
        object.groupId,
        specifiedType: const FullType(String),
      );
    }
    if (object.fromTargetKind != null) {
      yield r'from_target_kind';
      yield serializers.serialize(
        object.fromTargetKind,
        specifiedType: const FullType(TargetKind),
      );
    }
    if (object.fromTargetId != null) {
      yield r'from_target_id';
      yield serializers.serialize(
        object.fromTargetId,
        specifiedType: const FullType(String),
      );
    }
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.rule != null) {
      yield r'rule';
      yield serializers.serialize(
        object.rule,
        specifiedType: const FullType(AccessSourceRuleEnum),
      );
    }
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.membershipExpiresAt != null) {
      yield r'membership_expires_at';
      yield serializers.serialize(
        object.membershipExpiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessSource object, {
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
    required AccessSourceBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AccessSourceKindEnum),
          ) as AccessSourceKindEnum;
          result.kind = valueDes;
          break;
        case r'grant_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.grantId = valueDes;
          break;
        case r'group_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.groupId = valueDes;
          break;
        case r'from_target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TargetKind),
          ) as TargetKind?;
          if (valueDes == null) continue;
          result.fromTargetKind = valueDes;
          break;
        case r'from_target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.fromTargetId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'rule':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AccessSourceRuleEnum),
          ) as AccessSourceRuleEnum?;
          if (valueDes == null) continue;
          result.rule = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        case r'membership_expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.membershipExpiresAt = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessSource deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessSourceBuilder();
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

class AccessSourceKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'owner')
  static const AccessSourceKindEnum owner = _$accessSourceKindEnum_owner;
  @BuiltValueEnumConst(wireName: r'direct')
  static const AccessSourceKindEnum direct = _$accessSourceKindEnum_direct;
  @BuiltValueEnumConst(wireName: r'group')
  static const AccessSourceKindEnum group = _$accessSourceKindEnum_group;
  @BuiltValueEnumConst(wireName: r'inherited')
  static const AccessSourceKindEnum inherited =
      _$accessSourceKindEnum_inherited;
  @BuiltValueEnumConst(wireName: r'dm_participant')
  static const AccessSourceKindEnum dmParticipant =
      _$accessSourceKindEnum_dmParticipant;
  @BuiltValueEnumConst(wireName: r'operator')
  static const AccessSourceKindEnum operator_ =
      _$accessSourceKindEnum_operator_;
  @BuiltValueEnumConst(wireName: r'via_agent')
  static const AccessSourceKindEnum viaAgent = _$accessSourceKindEnum_viaAgent;
  @BuiltValueEnumConst(wireName: r'guard_blocked')
  static const AccessSourceKindEnum guardBlocked =
      _$accessSourceKindEnum_guardBlocked;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AccessSourceKindEnum unknownDefaultOpenApi =
      _$accessSourceKindEnum_unknownDefaultOpenApi;

  static Serializer<AccessSourceKindEnum> get serializer =>
      _$accessSourceKindEnumSerializer;

  const AccessSourceKindEnum._(String name) : super(name);

  static BuiltSet<AccessSourceKindEnum> get values =>
      _$accessSourceKindEnumValues;
  static AccessSourceKindEnum valueOf(String name) =>
      _$accessSourceKindEnumValueOf(name);
}

/// Guard rule of a `guard_blocked` source.
class AccessSourceRuleEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'sr2')
  static const AccessSourceRuleEnum sr2 = _$accessSourceRuleEnum_sr2;
  @BuiltValueEnumConst(wireName: r'sr3')
  static const AccessSourceRuleEnum sr3 = _$accessSourceRuleEnum_sr3;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AccessSourceRuleEnum unknownDefaultOpenApi =
      _$accessSourceRuleEnum_unknownDefaultOpenApi;

  static Serializer<AccessSourceRuleEnum> get serializer =>
      _$accessSourceRuleEnumSerializer;

  const AccessSourceRuleEnum._(String name) : super(name);

  static BuiltSet<AccessSourceRuleEnum> get values =>
      _$accessSourceRuleEnumValues;
  static AccessSourceRuleEnum valueOf(String name) =>
      _$accessSourceRuleEnumValueOf(name);
}
