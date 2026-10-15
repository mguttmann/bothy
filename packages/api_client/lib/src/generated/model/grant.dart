//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:api_client/src/generated/model/grant_subject_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'grant.g.dart';

/// Grant
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [subjectKind]
/// * [subjectId] - Principal ID for `user`, `agent`, `app`; group ID for `group`.
/// * [targetKind]
/// * [targetId] - Absent for `instance`.
/// * [level] - Absent exactly for `target_kind = host`.
/// * [capabilities]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [reason]
/// * [grantedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [grantedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [revokedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [revokedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [revokeReason]
@BuiltValue()
abstract class Grant implements Built<Grant, GrantBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'subject_kind')
  GrantSubjectKind get subjectKind;
  // enum subjectKindEnum {  user,  group,  agent,  app,  };

  /// Principal ID for `user`, `agent`, `app`; group ID for `group`.
  @BuiltValueField(wireName: r'subject_id')
  String get subjectId;

  @BuiltValueField(wireName: r'target_kind')
  TargetKind get targetKind;
  // enum targetKindEnum {  instance,  space,  channel,  host,  };

  /// Absent for `instance`.
  @BuiltValueField(wireName: r'target_id')
  String? get targetId;

  /// Absent exactly for `target_kind = host`.
  @BuiltValueField(wireName: r'level')
  Level? get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability> get capabilities;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'granted_by')
  String get grantedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'granted_at')
  DateTime get grantedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'revoked_at')
  DateTime? get revokedAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'revoked_by')
  String? get revokedBy;

  @BuiltValueField(wireName: r'revoke_reason')
  String? get revokeReason;

  Grant._();

  factory Grant([void updates(GrantBuilder b)]) = _$Grant;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GrantBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Grant> get serializer => _$GrantSerializer();
}

class _$GrantSerializer implements PrimitiveSerializer<Grant> {
  @override
  final Iterable<Type> types = const [Grant, _$Grant];

  @override
  final String wireName = r'Grant';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Grant object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'subject_kind';
    yield serializers.serialize(
      object.subjectKind,
      specifiedType: const FullType(GrantSubjectKind),
    );
    yield r'subject_id';
    yield serializers.serialize(
      object.subjectId,
      specifiedType: const FullType(String),
    );
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(TargetKind),
    );
    if (object.targetId != null) {
      yield r'target_id';
      yield serializers.serialize(
        object.targetId,
        specifiedType: const FullType(String),
      );
    }
    if (object.level != null) {
      yield r'level';
      yield serializers.serialize(
        object.level,
        specifiedType: const FullType(Level),
      );
    }
    yield r'capabilities';
    yield serializers.serialize(
      object.capabilities,
      specifiedType: const FullType(BuiltList, [FullType(Capability)]),
    );
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
    yield r'granted_by';
    yield serializers.serialize(
      object.grantedBy,
      specifiedType: const FullType(String),
    );
    yield r'granted_at';
    yield serializers.serialize(
      object.grantedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.revokedAt != null) {
      yield r'revoked_at';
      yield serializers.serialize(
        object.revokedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.revokedBy != null) {
      yield r'revoked_by';
      yield serializers.serialize(
        object.revokedBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.revokeReason != null) {
      yield r'revoke_reason';
      yield serializers.serialize(
        object.revokeReason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Grant object, {
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
    required GrantBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'subject_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GrantSubjectKind),
          ) as GrantSubjectKind;
          result.subjectKind = valueDes;
          break;
        case r'subject_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.subjectId = valueDes;
          break;
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TargetKind),
          ) as TargetKind;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.targetId = valueDes;
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Level),
          ) as Level?;
          if (valueDes == null) continue;
          result.level = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Capability)]),
          ) as BuiltList<Capability>;
          result.capabilities.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        case r'granted_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.grantedBy = valueDes;
          break;
        case r'granted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.grantedAt = valueDes;
          break;
        case r'revoked_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.revokedAt = valueDes;
          break;
        case r'revoked_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.revokedBy = valueDes;
          break;
        case r'revoke_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.revokeReason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Grant deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GrantBuilder();
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
