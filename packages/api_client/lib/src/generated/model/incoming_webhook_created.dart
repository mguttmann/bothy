//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/incoming_webhook.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_created.g.dart';

/// IncomingWebhookCreated
///
/// Properties:
/// * [webhook]
/// * [url] - Full URL including the token. Shown once.
@BuiltValue()
abstract class IncomingWebhookCreated
    implements Built<IncomingWebhookCreated, IncomingWebhookCreatedBuilder> {
  @BuiltValueField(wireName: r'webhook')
  IncomingWebhook get webhook;

  /// Full URL including the token. Shown once.
  @BuiltValueField(wireName: r'url')
  String get url;

  IncomingWebhookCreated._();

  factory IncomingWebhookCreated([
    void updates(IncomingWebhookCreatedBuilder b),
  ]) = _$IncomingWebhookCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookCreated> get serializer =>
      _$IncomingWebhookCreatedSerializer();
}

class _$IncomingWebhookCreatedSerializer
    implements PrimitiveSerializer<IncomingWebhookCreated> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookCreated,
    _$IncomingWebhookCreated,
  ];

  @override
  final String wireName = r'IncomingWebhookCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'webhook';
    yield serializers.serialize(
      object.webhook,
      specifiedType: const FullType(IncomingWebhook),
    );
    yield r'url';
    yield serializers.serialize(
      object.url,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookCreated object, {
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
    required IncomingWebhookCreatedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'webhook':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(IncomingWebhook),
          ) as IncomingWebhook;
          result.webhook.replace(valueDes);
          break;
        case r'url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.url = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IncomingWebhookCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookCreatedBuilder();
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
