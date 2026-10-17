//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/notification_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'push_preview.g.dart';

/// Preview for the notification extension, fields depend on `notify.preview_mode` (13, 6.4).
///
/// Properties:
/// * [kind]
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [badge]
/// * [spaceName]
/// * [channelName]
/// * [threadTitle]
/// * [senderDisplayName]
/// * [excerpt]
/// * [attachmentCount]
/// * [imageUrl] - Fetch ticket for the `small` variant of the first image.
@BuiltValue()
abstract class PushPreview implements Built<PushPreview, PushPreviewBuilder> {
  @BuiltValueField(wireName: r'kind')
  NotificationKind get kind;
  // enum kindEnum {  message,  mention,  dm,  run_done,  run_failed,  input_needed,  security,  admin,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String? get messageId;

  @BuiltValueField(wireName: r'badge')
  int get badge;

  @BuiltValueField(wireName: r'space_name')
  String? get spaceName;

  @BuiltValueField(wireName: r'channel_name')
  String? get channelName;

  @BuiltValueField(wireName: r'thread_title')
  String? get threadTitle;

  @BuiltValueField(wireName: r'sender_display_name')
  String? get senderDisplayName;

  @BuiltValueField(wireName: r'excerpt')
  String? get excerpt;

  @BuiltValueField(wireName: r'attachment_count')
  int? get attachmentCount;

  /// Fetch ticket for the `small` variant of the first image.
  @BuiltValueField(wireName: r'image_url')
  String? get imageUrl;

  PushPreview._();

  factory PushPreview([void updates(PushPreviewBuilder b)]) = _$PushPreview;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PushPreviewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PushPreview> get serializer => _$PushPreviewSerializer();
}

class _$PushPreviewSerializer implements PrimitiveSerializer<PushPreview> {
  @override
  final Iterable<Type> types = const [PushPreview, _$PushPreview];

  @override
  final String wireName = r'PushPreview';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PushPreview object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    yield r'badge';
    yield serializers.serialize(
      object.badge,
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
    PushPreview object, {
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
    required PushPreviewBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'badge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.badge = valueDes;
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
  PushPreview deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PushPreviewBuilder();
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
