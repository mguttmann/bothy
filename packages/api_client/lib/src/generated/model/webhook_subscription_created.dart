//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/webhook_subscription.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_subscription_created.g.dart';

/// WebhookSubscriptionCreated
///
/// Properties:
/// * [subscription]
/// * [hmacSecret] - Standard Webhooks secret with prefix `whsec_`. Shown once.
@BuiltValue()
abstract class WebhookSubscriptionCreated
    implements
        Built<WebhookSubscriptionCreated, WebhookSubscriptionCreatedBuilder> {
  @BuiltValueField(wireName: r'subscription')
  WebhookSubscription get subscription;

  /// Standard Webhooks secret with prefix `whsec_`. Shown once.
  @BuiltValueField(wireName: r'hmac_secret')
  String? get hmacSecret;

  WebhookSubscriptionCreated._();

  factory WebhookSubscriptionCreated([
    void updates(WebhookSubscriptionCreatedBuilder b),
  ]) = _$WebhookSubscriptionCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookSubscriptionCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookSubscriptionCreated> get serializer =>
      _$WebhookSubscriptionCreatedSerializer();
}

class _$WebhookSubscriptionCreatedSerializer
    implements PrimitiveSerializer<WebhookSubscriptionCreated> {
  @override
  final Iterable<Type> types = const [
    WebhookSubscriptionCreated,
    _$WebhookSubscriptionCreated,
  ];

  @override
  final String wireName = r'WebhookSubscriptionCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookSubscriptionCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'subscription';
    yield serializers.serialize(
      object.subscription,
      specifiedType: const FullType(WebhookSubscription),
    );
    if (object.hmacSecret != null) {
      yield r'hmac_secret';
      yield serializers.serialize(
        object.hmacSecret,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionCreated object, {
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
    required WebhookSubscriptionCreatedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'subscription':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(WebhookSubscription),
          ) as WebhookSubscription;
          result.subscription.replace(valueDes);
          break;
        case r'hmac_secret':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.hmacSecret = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookSubscriptionCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookSubscriptionCreatedBuilder();
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
