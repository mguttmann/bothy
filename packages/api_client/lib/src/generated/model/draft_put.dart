//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'draft_put.g.dart';

/// DraftPut
///
/// Properties:
/// * [contentMd]
/// * [replyToMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [fileIds]
/// * [baseVersion] - Version the client edited; older than current sets `overwrote_newer`.
@BuiltValue()
abstract class DraftPut implements Built<DraftPut, DraftPutBuilder> {
  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'reply_to_message_id')
  String? get replyToMessageId;

  @BuiltValueField(wireName: r'file_ids')
  BuiltList<String>? get fileIds;

  /// Version the client edited; older than current sets `overwrote_newer`.
  @BuiltValueField(wireName: r'base_version')
  int? get baseVersion;

  DraftPut._();

  factory DraftPut([void updates(DraftPutBuilder b)]) = _$DraftPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DraftPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DraftPut> get serializer => _$DraftPutSerializer();
}

class _$DraftPutSerializer implements PrimitiveSerializer<DraftPut> {
  @override
  final Iterable<Type> types = const [DraftPut, _$DraftPut];

  @override
  final String wireName = r'DraftPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DraftPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    if (object.replyToMessageId != null) {
      yield r'reply_to_message_id';
      yield serializers.serialize(
        object.replyToMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.fileIds != null) {
      yield r'file_ids';
      yield serializers.serialize(
        object.fileIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.baseVersion != null) {
      yield r'base_version';
      yield serializers.serialize(
        object.baseVersion,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    DraftPut object, {
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
    required DraftPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        case r'reply_to_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.replyToMessageId = valueDes;
          break;
        case r'file_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.fileIds.replace(valueDes);
          break;
        case r'base_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.baseVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DraftPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DraftPutBuilder();
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
