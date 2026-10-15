//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_object.g.dart';

/// ImportObject
///
/// Properties:
/// * [originKey]
/// * [itemKind]
/// * [contentSha256] - Hex SHA-256 of the converted content; a different value on a later run is counted as changed at the source.
/// * [data] - Fields of the target object, schema `import_object.<item_kind>.v1` (22, sections 5 and 6).
@BuiltValue()
abstract class ImportObject
    implements Built<ImportObject, ImportObjectBuilder> {
  @BuiltValueField(wireName: r'origin_key')
  String get originKey;

  @BuiltValueField(wireName: r'item_kind')
  ImportObjectItemKindEnum get itemKind;
  // enum itemKindEnum {  space,  channel,  forum_tag,  thread,  message,  user,  pin,  };

  /// Hex SHA-256 of the converted content; a different value on a later run is counted as changed at the source.
  @BuiltValueField(wireName: r'content_sha256')
  String? get contentSha256;

  /// Fields of the target object, schema `import_object.<item_kind>.v1` (22, sections 5 and 6).
  @BuiltValueField(wireName: r'data')
  BuiltMap<String, JsonObject?> get data;

  ImportObject._();

  factory ImportObject([void updates(ImportObjectBuilder b)]) = _$ImportObject;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportObjectBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportObject> get serializer => _$ImportObjectSerializer();
}

class _$ImportObjectSerializer implements PrimitiveSerializer<ImportObject> {
  @override
  final Iterable<Type> types = const [ImportObject, _$ImportObject];

  @override
  final String wireName = r'ImportObject';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportObject object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'origin_key';
    yield serializers.serialize(
      object.originKey,
      specifiedType: const FullType(String),
    );
    yield r'item_kind';
    yield serializers.serialize(
      object.itemKind,
      specifiedType: const FullType(ImportObjectItemKindEnum),
    );
    if (object.contentSha256 != null) {
      yield r'content_sha256';
      yield serializers.serialize(
        object.contentSha256,
        specifiedType: const FullType(String),
      );
    }
    yield r'data';
    yield serializers.serialize(
      object.data,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportObject object, {
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
    required ImportObjectBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.originKey = valueDes;
          break;
        case r'item_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportObjectItemKindEnum),
          ) as ImportObjectItemKindEnum;
          result.itemKind = valueDes;
          break;
        case r'content_sha256':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.contentSha256 = valueDes;
          break;
        case r'data':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.data.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportObject deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportObjectBuilder();
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

class ImportObjectItemKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'space')
  static const ImportObjectItemKindEnum space =
      _$importObjectItemKindEnum_space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const ImportObjectItemKindEnum channel =
      _$importObjectItemKindEnum_channel;
  @BuiltValueEnumConst(wireName: r'forum_tag')
  static const ImportObjectItemKindEnum forumTag =
      _$importObjectItemKindEnum_forumTag;
  @BuiltValueEnumConst(wireName: r'thread')
  static const ImportObjectItemKindEnum thread =
      _$importObjectItemKindEnum_thread;
  @BuiltValueEnumConst(wireName: r'message')
  static const ImportObjectItemKindEnum message =
      _$importObjectItemKindEnum_message;
  @BuiltValueEnumConst(wireName: r'user')
  static const ImportObjectItemKindEnum user = _$importObjectItemKindEnum_user;
  @BuiltValueEnumConst(wireName: r'pin')
  static const ImportObjectItemKindEnum pin = _$importObjectItemKindEnum_pin;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportObjectItemKindEnum unknownDefaultOpenApi =
      _$importObjectItemKindEnum_unknownDefaultOpenApi;

  static Serializer<ImportObjectItemKindEnum> get serializer =>
      _$importObjectItemKindEnumSerializer;

  const ImportObjectItemKindEnum._(String name) : super(name);

  static BuiltSet<ImportObjectItemKindEnum> get values =>
      _$importObjectItemKindEnumValues;
  static ImportObjectItemKindEnum valueOf(String name) =>
      _$importObjectItemKindEnumValueOf(name);
}
