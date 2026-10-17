//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space_create.g.dart';

/// SpaceCreate
///
/// Properties:
/// * [name] - At most `space.name_max_chars` characters.
/// * [icon] - Glyph name from the list `space-icons` of the design tokens.
/// * [color] - Key of a category color of the design system, never a hex value.
/// * [afterSpaceId] - Insert after this space; absent inserts at the end.
@BuiltValue()
abstract class SpaceCreate implements Built<SpaceCreate, SpaceCreateBuilder> {
  /// At most `space.name_max_chars` characters.
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Glyph name from the list `space-icons` of the design tokens.
  @BuiltValueField(wireName: r'icon')
  String? get icon;

  /// Key of a category color of the design system, never a hex value.
  @BuiltValueField(wireName: r'color')
  String? get color;

  /// Insert after this space; absent inserts at the end.
  @BuiltValueField(wireName: r'after_space_id')
  String? get afterSpaceId;

  SpaceCreate._();

  factory SpaceCreate([void updates(SpaceCreateBuilder b)]) = _$SpaceCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpaceCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SpaceCreate> get serializer => _$SpaceCreateSerializer();
}

class _$SpaceCreateSerializer implements PrimitiveSerializer<SpaceCreate> {
  @override
  final Iterable<Type> types = const [SpaceCreate, _$SpaceCreate];

  @override
  final String wireName = r'SpaceCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SpaceCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
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
    if (object.afterSpaceId != null) {
      yield r'after_space_id';
      yield serializers.serialize(
        object.afterSpaceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SpaceCreate object, {
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
    required SpaceCreateBuilder result,
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
        case r'after_space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.afterSpaceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SpaceCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpaceCreateBuilder();
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
