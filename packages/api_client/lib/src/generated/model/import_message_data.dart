//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_message_data.g.dart';

/// Data of a `message` item (22, 6.2). The message gets `origin = import`, `kind = text`, `silent`, no mentions and no `trigger_state`; `content_md` is stored as given (at most `message.max_content_bytes`, never split); the reply target must be a posted message of the same thread; the files must be posted file items of this source uploaded for the channel of the thread by the author, at most `message.max_attachments`.
///
/// Properties:
/// * [authorOriginKey]
/// * [contentMd]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [editedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [replyToOriginKey]
/// * [attachmentFileIds]
@BuiltValue()
abstract class ImportMessageData
    implements Built<ImportMessageData, ImportMessageDataBuilder> {
  @BuiltValueField(wireName: r'author_origin_key')
  String get authorOriginKey;

  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'edited_at')
  DateTime? get editedAt;

  @BuiltValueField(wireName: r'reply_to_origin_key')
  String? get replyToOriginKey;

  @BuiltValueField(wireName: r'attachment_file_ids')
  BuiltList<String>? get attachmentFileIds;

  ImportMessageData._();

  factory ImportMessageData([void updates(ImportMessageDataBuilder b)]) =
      _$ImportMessageData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportMessageDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportMessageData> get serializer =>
      _$ImportMessageDataSerializer();
}

class _$ImportMessageDataSerializer
    implements PrimitiveSerializer<ImportMessageData> {
  @override
  final Iterable<Type> types = const [ImportMessageData, _$ImportMessageData];

  @override
  final String wireName = r'ImportMessageData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportMessageData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'author_origin_key';
    yield serializers.serialize(
      object.authorOriginKey,
      specifiedType: const FullType(String),
    );
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.editedAt != null) {
      yield r'edited_at';
      yield serializers.serialize(
        object.editedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.replyToOriginKey != null) {
      yield r'reply_to_origin_key';
      yield serializers.serialize(
        object.replyToOriginKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.attachmentFileIds != null) {
      yield r'attachment_file_ids';
      yield serializers.serialize(
        object.attachmentFileIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportMessageData object, {
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
    required ImportMessageDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'author_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.authorOriginKey = valueDes;
          break;
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'edited_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.editedAt = valueDes;
          break;
        case r'reply_to_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.replyToOriginKey = valueDes;
          break;
        case r'attachment_file_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.attachmentFileIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportMessageData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportMessageDataBuilder();
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
