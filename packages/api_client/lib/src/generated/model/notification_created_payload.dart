//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/notification_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_created_payload.g.dart';

/// NotificationCreatedPayload
///
/// Properties:
/// * [nid] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [quiet] - Inside quiet hours; show without sound.
/// * [count] - Bundled notifications of this thread.
/// * [spaceName]
/// * [channelName]
/// * [threadTitle]
/// * [senderDisplayName]
/// * [excerpt] - Plain text, at most `notify.preview_max_chars`; only with `notify.preview_mode = full`.
/// * [attachmentCount]
/// * [imageUrl] - Fetch ticket for the `small` variant of the first image.
@BuiltValue()
abstract class NotificationCreatedPayload
    implements
        Built<NotificationCreatedPayload, NotificationCreatedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'nid')
  String get nid;

  @BuiltValueField(wireName: r'kind')
  NotificationKind get kind;
  // enum kindEnum {  message,  mention,  dm,  run_done,  run_failed,  input_needed,  security,  admin,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String? get messageId;

  /// Inside quiet hours; show without sound.
  @BuiltValueField(wireName: r'quiet')
  bool get quiet;

  /// Bundled notifications of this thread.
  @BuiltValueField(wireName: r'count')
  int get count;

  @BuiltValueField(wireName: r'space_name')
  String? get spaceName;

  @BuiltValueField(wireName: r'channel_name')
  String? get channelName;

  @BuiltValueField(wireName: r'thread_title')
  String? get threadTitle;

  @BuiltValueField(wireName: r'sender_display_name')
  String? get senderDisplayName;

  /// Plain text, at most `notify.preview_max_chars`; only with `notify.preview_mode = full`.
  @BuiltValueField(wireName: r'excerpt')
  String? get excerpt;

  @BuiltValueField(wireName: r'attachment_count')
  int? get attachmentCount;

  /// Fetch ticket for the `small` variant of the first image.
  @BuiltValueField(wireName: r'image_url')
  String? get imageUrl;

  NotificationCreatedPayload._();

  factory NotificationCreatedPayload([
    void updates(NotificationCreatedPayloadBuilder b),
  ]) = _$NotificationCreatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(NotificationCreatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<NotificationCreatedPayload> get serializer =>
      _$NotificationCreatedPayloadSerializer();
}

class _$NotificationCreatedPayloadSerializer
    implements PrimitiveSerializer<NotificationCreatedPayload> {
  @override
  final Iterable<Type> types = const [
    NotificationCreatedPayload,
    _$NotificationCreatedPayload,
  ];

  @override
  final String wireName = r'NotificationCreatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    NotificationCreatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'nid';
    yield serializers.serialize(
      object.nid,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(NotificationKind),
    );
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.messageId != null) {
      yield r'message_id';
      yield serializers.serialize(
        object.messageId,
        specifiedType: const FullType(String),
      );
    }
    yield r'quiet';
    yield serializers.serialize(
      object.quiet,
      specifiedType: const FullType(bool),
    );
    yield r'count';
    yield serializers.serialize(
      object.count,
      specifiedType: const FullType(int),
    );
    if (object.spaceName != null) {
      yield r'space_name';
      yield serializers.serialize(
        object.spaceName,
        specifiedType: const FullType(String),
      );
    }
    if (object.channelName != null) {
      yield r'channel_name';
      yield serializers.serialize(
        object.channelName,
        specifiedType: const FullType(String),
      );
    }
    if (object.threadTitle != null) {
      yield r'thread_title';
      yield serializers.serialize(
        object.threadTitle,
        specifiedType: const FullType(String),
      );
    }
    if (object.senderDisplayName != null) {
      yield r'sender_display_name';
      yield serializers.serialize(
        object.senderDisplayName,
        specifiedType: const FullType(String),
      );
    }
    if (object.excerpt != null) {
      yield r'excerpt';
      yield serializers.serialize(
        object.excerpt,
        specifiedType: const FullType(String),
      );
    }
    if (object.attachmentCount != null) {
      yield r'attachment_count';
      yield serializers.serialize(
        object.attachmentCount,
        specifiedType: const FullType(int),
      );
    }
    if (object.imageUrl != null) {
      yield r'image_url';
      yield serializers.serialize(
        object.imageUrl,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    NotificationCreatedPayload object, {
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
    required NotificationCreatedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'nid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.nid = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(NotificationKind),
          ) as NotificationKind;
          result.kind = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.messageId = valueDes;
          break;
        case r'quiet':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.quiet = valueDes;
          break;
        case r'count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.count = valueDes;
          break;
        case r'space_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceName = valueDes;
          break;
        case r'channel_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.channelName = valueDes;
          break;
        case r'thread_title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadTitle = valueDes;
          break;
        case r'sender_display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.senderDisplayName = valueDes;
          break;
        case r'excerpt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.excerpt = valueDes;
          break;
        case r'attachment_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.attachmentCount = valueDes;
          break;
        case r'image_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.imageUrl = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  NotificationCreatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = NotificationCreatedPayloadBuilder();
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
