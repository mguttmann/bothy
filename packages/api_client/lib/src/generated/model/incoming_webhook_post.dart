//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_post.g.dart';

/// IncomingWebhookPost
///
/// Properties:
/// * [contentMd]
/// * [clientMsgId] - Optional; makes retries of the sender idempotent.
/// * [title] - Title of a new thread when the webhook has no fixed thread.
@BuiltValue()
abstract class IncomingWebhookPost
    implements Built<IncomingWebhookPost, IncomingWebhookPostBuilder> {
  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  /// Optional; makes retries of the sender idempotent.
  @BuiltValueField(wireName: r'client_msg_id')
  String? get clientMsgId;

  /// Title of a new thread when the webhook has no fixed thread.
  @BuiltValueField(wireName: r'title')
  String? get title;

  IncomingWebhookPost._();

  factory IncomingWebhookPost([void updates(IncomingWebhookPostBuilder b)]) =
      _$IncomingWebhookPost;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookPostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookPost> get serializer =>
      _$IncomingWebhookPostSerializer();
}

class _$IncomingWebhookPostSerializer
    implements PrimitiveSerializer<IncomingWebhookPost> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookPost,
    _$IncomingWebhookPost,
  ];

  @override
  final String wireName = r'IncomingWebhookPost';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookPost object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    if (object.clientMsgId != null) {
      yield r'client_msg_id';
      yield serializers.serialize(
        object.clientMsgId,
        specifiedType: const FullType(String),
      );
    }
    if (object.title != null) {
      yield r'title';
      yield serializers.serialize(
        object.title,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookPost object, {
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
    required IncomingWebhookPostBuilder result,
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
        case r'client_msg_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.clientMsgId = valueDes;
          break;
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.title = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IncomingWebhookPost deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookPostBuilder();
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
