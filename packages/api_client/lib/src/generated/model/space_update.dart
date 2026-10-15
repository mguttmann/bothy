//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space_update.g.dart';

/// Changes only the fields sent; `icon` and `color` cannot be cleared.
///
/// Properties:
/// * [name] - At most `space.name_max_chars` characters.
/// * [icon] - Glyph name from the list `space-icons` of the design tokens.
/// * [color] - Key of a category colour of the design system, never a hex value.
@BuiltValue()
abstract class SpaceUpdate implements Built<SpaceUpdate, SpaceUpdateBuilder> {
  /// At most `space.name_max_chars` characters.
  @BuiltValueField(wireName: r'name')
  String? get name;

  /// Glyph name from the list `space-icons` of the design tokens.
  @BuiltValueField(wireName: r'icon')
  String? get icon;

  /// Key of a category colour of the design system, never a hex value.
  @BuiltValueField(wireName: r'color')
  String? get color;

  SpaceUpdate._();

  factory SpaceUpdate([void updates(SpaceUpdateBuilder b)]) = _$SpaceUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpaceUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SpaceUpdate> get serializer => _$SpaceUpdateSerializer();
}

class _$SpaceUpdateSerializer implements PrimitiveSerializer<SpaceUpdate> {
  @override
  final Iterable<Type> types = const [SpaceUpdate, _$SpaceUpdate];

  @override
  final String wireName = r'SpaceUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SpaceUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    if (object.icon != null) {
      yield r'icon';
      yield serializers.serialize(
        object.icon,
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
  }

  @override
  Object serialize(
    Serializers serializers,
    SpaceUpdate object, {
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
    required SpaceUpdateBuilder result,
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
        case r'icon':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.icon = valueDes;
          break;
        case r'color':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.color = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SpaceUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpaceUpdateBuilder();
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
