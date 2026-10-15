//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/message_create.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_create.g.dart';

/// Creates a forum post (with `title` and `first_message`) or a thread from a message of the main thread (with `parent_message_id`, optional `first_message`).
///
/// Properties:
/// * [title]
/// * [parentMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [tagIds] - Forum posts only; at most `thread.max_tags`. Restricted tags need `manage`, as with `setThreadTags`.
/// * [firstMessage]
@BuiltValue()
abstract class ThreadCreate
    implements Built<ThreadCreate, ThreadCreateBuilder> {
  @BuiltValueField(wireName: r'title')
  String? get title;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'parent_message_id')
  String? get parentMessageId;

  /// Forum posts only; at most `thread.max_tags`. Restricted tags need `manage`, as with `setThreadTags`.
  @BuiltValueField(wireName: r'tag_ids')
  BuiltList<String>? get tagIds;

  @BuiltValueField(wireName: r'first_message')
  MessageCreate? get firstMessage;

  ThreadCreate._();

  factory ThreadCreate([void updates(ThreadCreateBuilder b)]) = _$ThreadCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadCreate> get serializer => _$ThreadCreateSerializer();
}

class _$ThreadCreateSerializer implements PrimitiveSerializer<ThreadCreate> {
  @override
  final Iterable<Type> types = const [ThreadCreate, _$ThreadCreate];

  @override
  final String wireName = r'ThreadCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.title != null) {
      yield r'title';
      yield serializers.serialize(
        object.title,
        specifiedType: const FullType(String),
      );
    }
    if (object.parentMessageId != null) {
      yield r'parent_message_id';
      yield serializers.serialize(
        object.parentMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.tagIds != null) {
      yield r'tag_ids';
      yield serializers.serialize(
        object.tagIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.firstMessage != null) {
      yield r'first_message';
      yield serializers.serialize(
        object.firstMessage,
        specifiedType: const FullType(MessageCreate),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadCreate object, {
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
    required ThreadCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.title = valueDes;
          break;
        case r'parent_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.parentMessageId = valueDes;
          break;
        case r'tag_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.tagIds.replace(valueDes);
          break;
        case r'first_message':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(MessageCreate),
          ) as MessageCreate?;
          if (valueDes == null) continue;
          result.firstMessage.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadCreateBuilder();
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
