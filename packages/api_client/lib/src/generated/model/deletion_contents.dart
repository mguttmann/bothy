//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'deletion_contents.g.dart';

/// What the deletion removes, counted when read; for a pending or final deletion what it removed or removes.
///
/// Properties:
/// * [channels] - Channels and forums, not counting dm channels.
/// * [threads] - Chats and threads, not counting main threads.
/// * [messages]
/// * [files] - Files attached to those messages.
/// * [sessions] - Agent sessions of those chats.
@BuiltValue()
abstract class DeletionContents
    implements Built<DeletionContents, DeletionContentsBuilder> {
  /// Channels and forums, not counting dm channels.
  @BuiltValueField(wireName: r'channels')
  int get channels;

  /// Chats and threads, not counting main threads.
  @BuiltValueField(wireName: r'threads')
  int get threads;

  @BuiltValueField(wireName: r'messages')
  int get messages;

  /// Files attached to those messages.
  @BuiltValueField(wireName: r'files')
  int get files;

  /// Agent sessions of those chats.
  @BuiltValueField(wireName: r'sessions')
  int get sessions;

  DeletionContents._();

  factory DeletionContents([void updates(DeletionContentsBuilder b)]) =
      _$DeletionContents;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeletionContentsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DeletionContents> get serializer =>
      _$DeletionContentsSerializer();
}

class _$DeletionContentsSerializer
    implements PrimitiveSerializer<DeletionContents> {
  @override
  final Iterable<Type> types = const [DeletionContents, _$DeletionContents];

  @override
  final String wireName = r'DeletionContents';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DeletionContents object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channels';
    yield serializers.serialize(
      object.channels,
      specifiedType: const FullType(int),
    );
    yield r'threads';
    yield serializers.serialize(
      object.threads,
      specifiedType: const FullType(int),
    );
    yield r'messages';
    yield serializers.serialize(
      object.messages,
      specifiedType: const FullType(int),
    );
    yield r'files';
    yield serializers.serialize(
      object.files,
      specifiedType: const FullType(int),
    );
    yield r'sessions';
    yield serializers.serialize(
      object.sessions,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DeletionContents object, {
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
    required DeletionContentsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.channels = valueDes;
          break;
        case r'threads':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.threads = valueDes;
          break;
        case r'messages':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.messages = valueDes;
          break;
        case r'files':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.files = valueDes;
          break;
        case r'sessions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.sessions = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DeletionContents deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeletionContentsBuilder();
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
