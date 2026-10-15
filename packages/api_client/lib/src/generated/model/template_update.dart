//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'template_update.g.dart';

/// TemplateUpdate
///
/// Properties:
/// * [name]
/// * [contentMd]
@BuiltValue()
abstract class TemplateUpdate
    implements Built<TemplateUpdate, TemplateUpdateBuilder> {
  @BuiltValueField(wireName: r'name')
  String? get name;

  @BuiltValueField(wireName: r'content_md')
  String? get contentMd;

  TemplateUpdate._();

  factory TemplateUpdate([void updates(TemplateUpdateBuilder b)]) =
      _$TemplateUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TemplateUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TemplateUpdate> get serializer =>
      _$TemplateUpdateSerializer();
}

class _$TemplateUpdateSerializer
    implements PrimitiveSerializer<TemplateUpdate> {
  @override
  final Iterable<Type> types = const [TemplateUpdate, _$TemplateUpdate];

  @override
  final String wireName = r'TemplateUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TemplateUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    if (object.contentMd != null) {
      yield r'content_md';
      yield serializers.serialize(
        object.contentMd,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    TemplateUpdate object, {
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
    required TemplateUpdateBuilder result,
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
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  TemplateUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TemplateUpdateBuilder();
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
