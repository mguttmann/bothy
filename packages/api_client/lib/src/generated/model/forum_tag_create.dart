//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'forum_tag_create.g.dart';

/// ForumTagCreate
///
/// Properties:
/// * [name]
/// * [color] - Key of a category color of the design system, never a hex value.
/// * [restricted]
/// * [afterTagId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ForumTagCreate
    implements Built<ForumTagCreate, ForumTagCreateBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Key of a category color of the design system, never a hex value.
  @BuiltValueField(wireName: r'color')
  String? get color;

  @BuiltValueField(wireName: r'restricted')
  bool? get restricted;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'after_tag_id')
  String? get afterTagId;

  ForumTagCreate._();

  factory ForumTagCreate([void updates(ForumTagCreateBuilder b)]) =
      _$ForumTagCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ForumTagCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ForumTagCreate> get serializer =>
      _$ForumTagCreateSerializer();
}

class _$ForumTagCreateSerializer
    implements PrimitiveSerializer<ForumTagCreate> {
  @override
  final Iterable<Type> types = const [ForumTagCreate, _$ForumTagCreate];

  @override
  final String wireName = r'ForumTagCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ForumTagCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.color != null) {
      yield r'color';
      yield serializers.serialize(
        object.color,
        specifiedType: const FullType(String),
      );
    }
    if (object.restricted != null) {
      yield r'restricted';
      yield serializers.serialize(
        object.restricted,
        specifiedType: const FullType(bool),
      );
    }
    if (object.afterTagId != null) {
      yield r'after_tag_id';
      yield serializers.serialize(
        object.afterTagId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ForumTagCreate object, {
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
    required ForumTagCreateBuilder result,
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
        case r'color':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.color = valueDes;
          break;
        case r'restricted':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.restricted = valueDes;
          break;
        case r'after_tag_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.afterTagId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ForumTagCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ForumTagCreateBuilder();
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
