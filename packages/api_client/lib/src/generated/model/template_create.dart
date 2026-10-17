//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'template_create.g.dart';

/// TemplateCreate
///
/// Properties:
/// * [name]
/// * [contentMd]
@BuiltValue()
abstract class TemplateCreate
    implements Built<TemplateCreate, TemplateCreateBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  TemplateCreate._();

  factory TemplateCreate([void updates(TemplateCreateBuilder b)]) =
      _$TemplateCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TemplateCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TemplateCreate> get serializer =>
      _$TemplateCreateSerializer();
}

class _$TemplateCreateSerializer
    implements PrimitiveSerializer<TemplateCreate> {
  @override
  final Iterable<Type> types = const [TemplateCreate, _$TemplateCreate];

  @override
  final String wireName = r'TemplateCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TemplateCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TemplateCreate object, {
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
    required TemplateCreateBuilder result,
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
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TemplateCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TemplateCreateBuilder();
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
