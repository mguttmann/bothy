//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_forum_tag_data.g.dart';

/// `data` of a `forum_tag` item (origin key `forum_tag:<channel_id>:<tag_id>`). A tag with the same name, without regard to case, is adopted (22, 6.6). `emoji` is accepted and not stored (forum tags have no emoji).
///
/// Properties:
/// * [channelOriginKey]
/// * [name]
/// * [emoji]
@BuiltValue()
abstract class ImportForumTagData
    implements Built<ImportForumTagData, ImportForumTagDataBuilder> {
  @BuiltValueField(wireName: r'channel_origin_key')
  String get channelOriginKey;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'emoji')
  String? get emoji;

  ImportForumTagData._();

  factory ImportForumTagData([void updates(ImportForumTagDataBuilder b)]) =
      _$ImportForumTagData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportForumTagDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportForumTagData> get serializer =>
      _$ImportForumTagDataSerializer();
}

class _$ImportForumTagDataSerializer
    implements PrimitiveSerializer<ImportForumTagData> {
  @override
  final Iterable<Type> types = const [ImportForumTagData, _$ImportForumTagData];

  @override
  final String wireName = r'ImportForumTagData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportForumTagData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_origin_key';
    yield serializers.serialize(
      object.channelOriginKey,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.emoji != null) {
      yield r'emoji';
      yield serializers.serialize(
        object.emoji,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportForumTagData object, {
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
    required ImportForumTagDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelOriginKey = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'emoji':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.emoji = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportForumTagData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportForumTagDataBuilder();
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
