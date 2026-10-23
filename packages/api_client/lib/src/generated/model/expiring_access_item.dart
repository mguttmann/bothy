//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/grant.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'expiring_access_item.g.dart';

/// One access that expires within `authz.expiry_reminder_lead`. `grant` carries the `grant`; `membership` names `membership_id`, `group_id` and `user_id`; `token` names `token_id` and the holder `principal_id` (an agent or an app), never the token value.
///
/// Properties:
/// * [kind]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [grant]
/// * [membershipId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [groupId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [tokenId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [principalId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ExpiringAccessItem
    implements Built<ExpiringAccessItem, ExpiringAccessItemBuilder> {
  @BuiltValueField(wireName: r'kind')
  ExpiringAccessItemKindEnum get kind;
  // enum kindEnum {  grant,  membership,  token,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  @BuiltValueField(wireName: r'grant')
  Grant? get grant;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'membership_id')
  String? get membershipId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'group_id')
  String? get groupId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String? get userId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'token_id')
  String? get tokenId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'principal_id')
  String? get principalId;

  ExpiringAccessItem._();

  factory ExpiringAccessItem([void updates(ExpiringAccessItemBuilder b)]) =
      _$ExpiringAccessItem;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ExpiringAccessItemBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ExpiringAccessItem> get serializer =>
      _$ExpiringAccessItemSerializer();
}

class _$ExpiringAccessItemSerializer
    implements PrimitiveSerializer<ExpiringAccessItem> {
  @override
  final Iterable<Type> types = const [ExpiringAccessItem, _$ExpiringAccessItem];

  @override
  final String wireName = r'ExpiringAccessItem';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ExpiringAccessItem object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ExpiringAccessItemKindEnum),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.grant != null) {
      yield r'grant';
      yield serializers.serialize(
        object.grant,
        specifiedType: const FullType(Grant),
      );
    }
    if (object.membershipId != null) {
      yield r'membership_id';
      yield serializers.serialize(
        object.membershipId,
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
    if (object.userId != null) {
      yield r'user_id';
      yield serializers.serialize(
        object.userId,
        specifiedType: const FullType(String),
      );
    }
    if (object.tokenId != null) {
      yield r'token_id';
      yield serializers.serialize(
        object.tokenId,
        specifiedType: const FullType(String),
      );
    }
    if (object.principalId != null) {
      yield r'principal_id';
      yield serializers.serialize(
        object.principalId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ExpiringAccessItem object, {
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
    required ExpiringAccessItemBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ExpiringAccessItemKindEnum),
          ) as ExpiringAccessItemKindEnum;
          result.kind = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        case r'grant':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Grant),
          ) as Grant?;
          if (valueDes == null) continue;
          result.grant.replace(valueDes);
          break;
        case r'membership_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.membershipId = valueDes;
          break;
        case r'group_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.groupId = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.userId = valueDes;
          break;
        case r'token_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.tokenId = valueDes;
          break;
        case r'principal_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.principalId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ExpiringAccessItem deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ExpiringAccessItemBuilder();
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

class ExpiringAccessItemKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'grant')
  static const ExpiringAccessItemKindEnum grant =
      _$expiringAccessItemKindEnum_grant;
  @BuiltValueEnumConst(wireName: r'membership')
  static const ExpiringAccessItemKindEnum membership =
      _$expiringAccessItemKindEnum_membership;
  @BuiltValueEnumConst(wireName: r'token')
  static const ExpiringAccessItemKindEnum token =
      _$expiringAccessItemKindEnum_token;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ExpiringAccessItemKindEnum unknownDefaultOpenApi =
      _$expiringAccessItemKindEnum_unknownDefaultOpenApi;

  static Serializer<ExpiringAccessItemKindEnum> get serializer =>
      _$expiringAccessItemKindEnumSerializer;

  const ExpiringAccessItemKindEnum._(String name) : super(name);

  static BuiltSet<ExpiringAccessItemKindEnum> get values =>
      _$expiringAccessItemKindEnumValues;
  static ExpiringAccessItemKindEnum valueOf(String name) =>
      _$expiringAccessItemKindEnumValueOf(name);
}
