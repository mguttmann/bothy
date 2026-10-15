//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'forum_tag_update.g.dart';

/// ForumTagUpdate
///
/// Properties:
/// * [name]
/// * [color] - Key of a category color of the design system, never a hex value.
/// * [clearColor] - Removes the color; not together with `color`. There is no JSON null (R-API-03).
/// * [restricted]
@BuiltValue()
abstract class ForumTagUpdate
    implements Built<ForumTagUpdate, ForumTagUpdateBuilder> {
  @BuiltValueField(wireName: r'name')
  String? get name;

  /// Key of a category color of the design system, never a hex value.
  @BuiltValueField(wireName: r'color')
  String? get color;

  /// Removes the color; not together with `color`. There is no JSON null (R-API-03).
  @BuiltValueField(wireName: r'clear_color')
  bool? get clearColor;

  @BuiltValueField(wireName: r'restricted')
  bool? get restricted;

  ForumTagUpdate._();

  factory ForumTagUpdate([void updates(ForumTagUpdateBuilder b)]) =
      _$ForumTagUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ForumTagUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ForumTagUpdate> get serializer =>
      _$ForumTagUpdateSerializer();
}

class _$ForumTagUpdateSerializer
    implements PrimitiveSerializer<ForumTagUpdate> {
  @override
  final Iterable<Type> types = const [ForumTagUpdate, _$ForumTagUpdate];

  @override
  final String wireName = r'ForumTagUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ForumTagUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    if (object.color != null) {
      yield r'color';
      yield serializers.serialize(
        object.color,
        specifiedType: const FullType(String),
      );
    }
    if (object.clearColor != null) {
      yield r'clear_color';
      yield serializers.serialize(
        object.clearColor,
        specifiedType: const FullType(bool),
      );
    }
    if (object.restricted != null) {
      yield r'restricted';
      yield serializers.serialize(
        object.restricted,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ForumTagUpdate object, {
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
    required ForumTagUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
        case r'clear_color':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.clearColor = valueDes;
          break;
        case r'restricted':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.restricted = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ForumTagUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ForumTagUpdateBuilder();
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
