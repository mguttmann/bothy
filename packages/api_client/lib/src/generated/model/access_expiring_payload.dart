//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/access_target.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_expiring_payload.g.dart';

/// A grant or group membership ends within `authz.expiry_reminder_lead`.
///
/// Properties:
/// * [kind]
/// * [grantId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [groupId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [subjectId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [target] - Absent for a grant on the instance.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class AccessExpiringPayload
    implements Built<AccessExpiringPayload, AccessExpiringPayloadBuilder> {
  @BuiltValueField(wireName: r'kind')
  AccessExpiringPayloadKindEnum get kind;
  // enum kindEnum {  grant,  group_membership,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'grant_id')
  String? get grantId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'group_id')
  String? get groupId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'subject_id')
  String get subjectId;

  /// Absent for a grant on the instance.
  @BuiltValueField(wireName: r'target')
  AccessTarget? get target;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  AccessExpiringPayload._();

  factory AccessExpiringPayload([
    void updates(AccessExpiringPayloadBuilder b),
  ]) = _$AccessExpiringPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessExpiringPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessExpiringPayload> get serializer =>
      _$AccessExpiringPayloadSerializer();
}

class _$AccessExpiringPayloadSerializer
    implements PrimitiveSerializer<AccessExpiringPayload> {
  @override
  final Iterable<Type> types = const [
    AccessExpiringPayload,
    _$AccessExpiringPayload,
  ];

  @override
  final String wireName = r'AccessExpiringPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessExpiringPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(AccessExpiringPayloadKindEnum),
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
    yield r'subject_id';
    yield serializers.serialize(
      object.subjectId,
      specifiedType: const FullType(String),
    );
    if (object.target != null) {
      yield r'target';
      yield serializers.serialize(
        object.target,
        specifiedType: const FullType(AccessTarget),
      );
    }
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessExpiringPayload object, {
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
    required AccessExpiringPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AccessExpiringPayloadKindEnum),
          ) as AccessExpiringPayloadKindEnum;
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
        case r'subject_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.subjectId = valueDes;
          break;
        case r'target':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AccessTarget),
          ) as AccessTarget?;
          if (valueDes == null) continue;
          result.target.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessExpiringPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessExpiringPayloadBuilder();
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

class AccessExpiringPayloadKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'grant')
  static const AccessExpiringPayloadKindEnum grant =
      _$accessExpiringPayloadKindEnum_grant;
  @BuiltValueEnumConst(wireName: r'group_membership')
  static const AccessExpiringPayloadKindEnum groupMembership =
      _$accessExpiringPayloadKindEnum_groupMembership;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AccessExpiringPayloadKindEnum unknownDefaultOpenApi =
      _$accessExpiringPayloadKindEnum_unknownDefaultOpenApi;

  static Serializer<AccessExpiringPayloadKindEnum> get serializer =>
      _$accessExpiringPayloadKindEnumSerializer;

  const AccessExpiringPayloadKindEnum._(String name) : super(name);

  static BuiltSet<AccessExpiringPayloadKindEnum> get values =>
      _$accessExpiringPayloadKindEnumValues;
  static AccessExpiringPayloadKindEnum valueOf(String name) =>
      _$accessExpiringPayloadKindEnumValueOf(name);
}
