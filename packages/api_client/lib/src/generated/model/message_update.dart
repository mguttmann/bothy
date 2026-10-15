//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_update.g.dart';

/// The server resolves the mentions of the resulting content again; `mention_ids` is an optional check as in `MessageCreate`.
///
/// Properties:
/// * [contentMd]
/// * [mentionIds]
@BuiltValue()
abstract class MessageUpdate
    implements Built<MessageUpdate, MessageUpdateBuilder> {
  @BuiltValueField(wireName: r'content_md')
  String? get contentMd;

  @BuiltValueField(wireName: r'mention_ids')
  BuiltList<String>? get mentionIds;

  MessageUpdate._();

  factory MessageUpdate([void updates(MessageUpdateBuilder b)]) =
      _$MessageUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessageUpdate> get serializer =>
      _$MessageUpdateSerializer();
}

class _$MessageUpdateSerializer implements PrimitiveSerializer<MessageUpdate> {
  @override
  final Iterable<Type> types = const [MessageUpdate, _$MessageUpdate];

  @override
  final String wireName = r'MessageUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessageUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.contentMd != null) {
      yield r'content_md';
      yield serializers.serialize(
        object.contentMd,
        specifiedType: const FullType(String),
      );
    }
    if (object.mentionIds != null) {
      yield r'mention_ids';
      yield serializers.serialize(
        object.mentionIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    MessageUpdate object, {
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
    required MessageUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.contentMd = valueDes;
          break;
        case r'mention_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.mentionIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessageUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageUpdateBuilder();
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
