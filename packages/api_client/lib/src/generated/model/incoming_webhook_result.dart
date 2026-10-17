//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_result.g.dart';

/// IncomingWebhookResult
///
/// Properties:
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class IncomingWebhookResult
    implements Built<IncomingWebhookResult, IncomingWebhookResultBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  IncomingWebhookResult._();

  factory IncomingWebhookResult([
    void updates(IncomingWebhookResultBuilder b),
  ]) = _$IncomingWebhookResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookResult> get serializer =>
      _$IncomingWebhookResultSerializer();
}

class _$IncomingWebhookResultSerializer
    implements PrimitiveSerializer<IncomingWebhookResult> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookResult,
    _$IncomingWebhookResult,
  ];

  @override
  final String wireName = r'IncomingWebhookResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookResult object, {
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
    required IncomingWebhookResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IncomingWebhookResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookResultBuilder();
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
