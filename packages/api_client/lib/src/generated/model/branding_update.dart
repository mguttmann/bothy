//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'branding_update.g.dart';

/// BrandingUpdate
///
/// Properties:
/// * [productName]
/// * [accentColor]
@BuiltValue()
abstract class BrandingUpdate
    implements Built<BrandingUpdate, BrandingUpdateBuilder> {
  @BuiltValueField(wireName: r'product_name')
  String get productName;

  @BuiltValueField(wireName: r'accent_color')
  String? get accentColor;

  BrandingUpdate._();

  factory BrandingUpdate([void updates(BrandingUpdateBuilder b)]) =
      _$BrandingUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BrandingUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<BrandingUpdate> get serializer =>
      _$BrandingUpdateSerializer();
}

class _$BrandingUpdateSerializer
    implements PrimitiveSerializer<BrandingUpdate> {
  @override
  final Iterable<Type> types = const [BrandingUpdate, _$BrandingUpdate];

  @override
  final String wireName = r'BrandingUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    BrandingUpdate object, {
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
  }

  @override
  Object serialize(
    Serializers serializers,
    BrandingUpdate object, {
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
    required BrandingUpdateBuilder result,
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  BrandingUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BrandingUpdateBuilder();
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
