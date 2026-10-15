//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'branding.g.dart';

/// Branding
///
/// Properties:
/// * [productName]
/// * [accentColor]
/// * [logoFileId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [version]
@BuiltValue()
abstract class Branding implements Built<Branding, BrandingBuilder> {
  @BuiltValueField(wireName: r'product_name')
  String get productName;

  @BuiltValueField(wireName: r'accent_color')
  String? get accentColor;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'logo_file_id')
  String? get logoFileId;

  @BuiltValueField(wireName: r'version')
  int get version;

  Branding._();

  factory Branding([void updates(BrandingBuilder b)]) = _$Branding;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BrandingBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Branding> get serializer => _$BrandingSerializer();
}

class _$BrandingSerializer implements PrimitiveSerializer<Branding> {
  @override
  final Iterable<Type> types = const [Branding, _$Branding];

  @override
  final String wireName = r'Branding';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Branding object, {
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
    if (object.logoFileId != null) {
      yield r'logo_file_id';
      yield serializers.serialize(
        object.logoFileId,
        specifiedType: const FullType(String),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Branding object, {
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
    required BrandingBuilder result,
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
        case r'logo_file_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.logoFileId = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Branding deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BrandingBuilder();
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
