//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'template.g.dart';

/// Template
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [scope]
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [contentMd]
/// * [version]
@BuiltValue()
abstract class Template implements Built<Template, TemplateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'scope')
  TemplateScopeEnum get scope;
  // enum scopeEnum {  personal,  channel,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String? get channelId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  @BuiltValueField(wireName: r'version')
  int get version;

  Template._();

  factory Template([void updates(TemplateBuilder b)]) = _$Template;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TemplateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Template> get serializer => _$TemplateSerializer();
}

class _$TemplateSerializer implements PrimitiveSerializer<Template> {
  @override
  final Iterable<Type> types = const [Template, _$Template];

  @override
  final String wireName = r'Template';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Template object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'scope';
    yield serializers.serialize(
      object.scope,
      specifiedType: const FullType(TemplateScopeEnum),
    );
    if (object.channelId != null) {
      yield r'channel_id';
      yield serializers.serialize(
        object.channelId,
        specifiedType: const FullType(String),
      );
    }
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
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Template object, {
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
    required TemplateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'scope':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TemplateScopeEnum),
          ) as TemplateScopeEnum;
          result.scope = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.channelId = valueDes;
          break;
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
  Template deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TemplateBuilder();
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

class TemplateScopeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'personal')
  static const TemplateScopeEnum personal = _$templateScopeEnum_personal;
  @BuiltValueEnumConst(wireName: r'channel')
  static const TemplateScopeEnum channel = _$templateScopeEnum_channel;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TemplateScopeEnum unknownDefaultOpenApi =
      _$templateScopeEnum_unknownDefaultOpenApi;

  static Serializer<TemplateScopeEnum> get serializer =>
      _$templateScopeEnumSerializer;

  const TemplateScopeEnum._(String name) : super(name);

  static BuiltSet<TemplateScopeEnum> get values => _$templateScopeEnumValues;
  static TemplateScopeEnum valueOf(String name) =>
      _$templateScopeEnumValueOf(name);
}
