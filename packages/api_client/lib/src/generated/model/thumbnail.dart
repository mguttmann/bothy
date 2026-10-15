//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thumbnail.g.dart';

/// Thumbnail
///
/// Properties:
/// * [variant]
/// * [mimeType]
/// * [width]
/// * [height]
@BuiltValue()
abstract class Thumbnail implements Built<Thumbnail, ThumbnailBuilder> {
  @BuiltValueField(wireName: r'variant')
  ThumbnailVariantEnum get variant;
  // enum variantEnum {  small,  medium,  large,  poster,  };

  @BuiltValueField(wireName: r'mime_type')
  String get mimeType;

  @BuiltValueField(wireName: r'width')
  int get width;

  @BuiltValueField(wireName: r'height')
  int get height;

  Thumbnail._();

  factory Thumbnail([void updates(ThumbnailBuilder b)]) = _$Thumbnail;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThumbnailBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Thumbnail> get serializer => _$ThumbnailSerializer();
}

class _$ThumbnailSerializer implements PrimitiveSerializer<Thumbnail> {
  @override
  final Iterable<Type> types = const [Thumbnail, _$Thumbnail];

  @override
  final String wireName = r'Thumbnail';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Thumbnail object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'variant';
    yield serializers.serialize(
      object.variant,
      specifiedType: const FullType(ThumbnailVariantEnum),
    );
    yield r'mime_type';
    yield serializers.serialize(
      object.mimeType,
      specifiedType: const FullType(String),
    );
    yield r'width';
    yield serializers.serialize(
      object.width,
      specifiedType: const FullType(int),
    );
    yield r'height';
    yield serializers.serialize(
      object.height,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Thumbnail object, {
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
    required ThumbnailBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'variant':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ThumbnailVariantEnum),
          ) as ThumbnailVariantEnum;
          result.variant = valueDes;
          break;
        case r'mime_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.mimeType = valueDes;
          break;
        case r'width':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.width = valueDes;
          break;
        case r'height':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.height = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Thumbnail deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThumbnailBuilder();
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

class ThumbnailVariantEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'small')
  static const ThumbnailVariantEnum small = _$thumbnailVariantEnum_small;
  @BuiltValueEnumConst(wireName: r'medium')
  static const ThumbnailVariantEnum medium = _$thumbnailVariantEnum_medium;
  @BuiltValueEnumConst(wireName: r'large')
  static const ThumbnailVariantEnum large = _$thumbnailVariantEnum_large;
  @BuiltValueEnumConst(wireName: r'poster')
  static const ThumbnailVariantEnum poster = _$thumbnailVariantEnum_poster;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ThumbnailVariantEnum unknownDefaultOpenApi =
      _$thumbnailVariantEnum_unknownDefaultOpenApi;

  static Serializer<ThumbnailVariantEnum> get serializer =>
      _$thumbnailVariantEnumSerializer;

  const ThumbnailVariantEnum._(String name) : super(name);

  static BuiltSet<ThumbnailVariantEnum> get values =>
      _$thumbnailVariantEnumValues;
  static ThumbnailVariantEnum valueOf(String name) =>
      _$thumbnailVariantEnumValueOf(name);
}
