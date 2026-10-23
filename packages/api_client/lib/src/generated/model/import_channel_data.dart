//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_channel_data.g.dart';

/// `data` of a `channel` item (origin key `channel:<channel_id>`). The space is the target of the posted item `space_origin_key` or, without it, of the category named in `defaults.uncategorized_space`; `existing_space_id` and `existing_channel_id` must equal the stored mapping. `read_only` is not a member: the server derives it from the stored `lead`.
///
/// Properties:
/// * [spaceOriginKey]
/// * [existingSpaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [kind]
/// * [topic]
/// * [sortKey] - Fractional index key of the order in the space.
/// * [existingChannelId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ImportChannelData
    implements Built<ImportChannelData, ImportChannelDataBuilder> {
  @BuiltValueField(wireName: r'space_origin_key')
  String? get spaceOriginKey;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'existing_space_id')
  String? get existingSpaceId;

  @BuiltValueField(wireName: r'name')
  String? get name;

  @BuiltValueField(wireName: r'kind')
  ImportChannelDataKindEnum get kind;
  // enum kindEnum {  text,  forum,  };

  @BuiltValueField(wireName: r'topic')
  String? get topic;

  /// Fractional index key of the order in the space.
  @BuiltValueField(wireName: r'sort_key')
  String get sortKey;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'existing_channel_id')
  String? get existingChannelId;

  ImportChannelData._();

  factory ImportChannelData([void updates(ImportChannelDataBuilder b)]) =
      _$ImportChannelData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportChannelDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportChannelData> get serializer =>
      _$ImportChannelDataSerializer();
}

class _$ImportChannelDataSerializer
    implements PrimitiveSerializer<ImportChannelData> {
  @override
  final Iterable<Type> types = const [ImportChannelData, _$ImportChannelData];

  @override
  final String wireName = r'ImportChannelData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportChannelData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.spaceOriginKey != null) {
      yield r'space_origin_key';
      yield serializers.serialize(
        object.spaceOriginKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.existingSpaceId != null) {
      yield r'existing_space_id';
      yield serializers.serialize(
        object.existingSpaceId,
        specifiedType: const FullType(String),
      );
    }
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ImportChannelDataKindEnum),
    );
    if (object.topic != null) {
      yield r'topic';
      yield serializers.serialize(
        object.topic,
        specifiedType: const FullType(String),
      );
    }
    yield r'sort_key';
    yield serializers.serialize(
      object.sortKey,
      specifiedType: const FullType(String),
    );
    if (object.existingChannelId != null) {
      yield r'existing_channel_id';
      yield serializers.serialize(
        object.existingChannelId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportChannelData object, {
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
    required ImportChannelDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceOriginKey = valueDes;
          break;
        case r'existing_space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.existingSpaceId = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.name = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportChannelDataKindEnum),
          ) as ImportChannelDataKindEnum;
          result.kind = valueDes;
          break;
        case r'topic':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.topic = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sortKey = valueDes;
          break;
        case r'existing_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.existingChannelId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportChannelData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportChannelDataBuilder();
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

class ImportChannelDataKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'text')
  static const ImportChannelDataKindEnum text =
      _$importChannelDataKindEnum_text;
  @BuiltValueEnumConst(wireName: r'forum')
  static const ImportChannelDataKindEnum forum =
      _$importChannelDataKindEnum_forum;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportChannelDataKindEnum unknownDefaultOpenApi =
      _$importChannelDataKindEnum_unknownDefaultOpenApi;

  static Serializer<ImportChannelDataKindEnum> get serializer =>
      _$importChannelDataKindEnumSerializer;

  const ImportChannelDataKindEnum._(String name) : super(name);

  static BuiltSet<ImportChannelDataKindEnum> get values =>
      _$importChannelDataKindEnumValues;
  static ImportChannelDataKindEnum valueOf(String name) =>
      _$importChannelDataKindEnumValueOf(name);
}
