//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'public_branding.g.dart';

/// PublicBranding
///
/// Properties:
/// * [productName]
/// * [accentColor] - Key of a category color of the design system (`spruce`, `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss`), never a free color value (ADR 0044).
/// * [logoUrl]
@BuiltValue()
abstract class PublicBranding
    implements Built<PublicBranding, PublicBrandingBuilder> {
  @BuiltValueField(wireName: r'product_name')
  String get productName;

  /// Key of a category color of the design system (`spruce`, `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss`), never a free color value (ADR 0044).
  @BuiltValueField(wireName: r'accent_color')
  String? get accentColor;

  @BuiltValueField(wireName: r'logo_url')
  String? get logoUrl;

  PublicBranding._();

  factory PublicBranding([void updates(PublicBrandingBuilder b)]) =
      _$PublicBranding;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PublicBrandingBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PublicBranding> get serializer =>
      _$PublicBrandingSerializer();
}

class _$PublicBrandingSerializer
    implements PrimitiveSerializer<PublicBranding> {
  @override
  final Iterable<Type> types = const [PublicBranding, _$PublicBranding];

  @override
  final String wireName = r'PublicBranding';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PublicBranding object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'product_name';
    yield serializers.serialize(
      object.productName,
      specifiedType: const FullType(String),
    );
    if (object.accentColor != null) {
      yield r'accent_color';
      yield serializers.serialize(
        object.accentColor,
        specifiedType: const FullType(String),
      );
    }
    if (object.logoUrl != null) {
      yield r'logo_url';
      yield serializers.serialize(
        object.logoUrl,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PublicBranding object, {
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
    required PublicBrandingBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'product_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.productName = valueDes;
          break;
        case r'accent_color':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.accentColor = valueDes;
          break;
        case r'logo_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.logoUrl = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PublicBranding deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PublicBrandingBuilder();
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
