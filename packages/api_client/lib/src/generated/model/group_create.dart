//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/group_source.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'group_create.g.dart';

/// GroupCreate
///
/// Properties:
/// * [name]
/// * [description]
/// * [source_]
/// * [providerId] - Required with `source = oidc_claim`; the OIDC provider whose claim manages the members.
/// * [externalRef] - Required with `source = oidc_claim`; the claim value.
@BuiltValue()
abstract class GroupCreate implements Built<GroupCreate, GroupCreateBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String? get description;

  @BuiltValueField(wireName: r'source')
  GroupSource? get source_;
  // enum source_Enum {  local,  oidc_claim,  };

  /// Required with `source = oidc_claim`; the OIDC provider whose claim manages the members.
  @BuiltValueField(wireName: r'provider_id')
  String? get providerId;

  /// Required with `source = oidc_claim`; the claim value.
  @BuiltValueField(wireName: r'external_ref')
  String? get externalRef;

  GroupCreate._();

  factory GroupCreate([void updates(GroupCreateBuilder b)]) = _$GroupCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GroupCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GroupCreate> get serializer => _$GroupCreateSerializer();
}

class _$GroupCreateSerializer implements PrimitiveSerializer<GroupCreate> {
  @override
  final Iterable<Type> types = const [GroupCreate, _$GroupCreate];

  @override
  final String wireName = r'GroupCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GroupCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.source_ != null) {
      yield r'source';
      yield serializers.serialize(
        object.source_,
        specifiedType: const FullType(GroupSource),
      );
    }
    if (object.providerId != null) {
      yield r'provider_id';
      yield serializers.serialize(
        object.providerId,
        specifiedType: const FullType(String),
      );
    }
    if (object.externalRef != null) {
      yield r'external_ref';
      yield serializers.serialize(
        object.externalRef,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GroupCreate object, {
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
    required GroupCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(GroupSource),
          ) as GroupSource?;
          if (valueDes == null) continue;
          result.source_ = valueDes;
          break;
        case r'provider_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.providerId = valueDes;
          break;
        case r'external_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.externalRef = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GroupCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GroupCreateBuilder();
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
