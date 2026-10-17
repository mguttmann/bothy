//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_subscription_create.g.dart';

/// WebhookSubscriptionCreate
///
/// Properties:
/// * [url] - HTTPS URL; the host must be in `webhook.allowed_hosts`.
/// * [eventTypes]
/// * [enableHmac] - Also sign with Standard Webhooks `v1` (HMAC); the secret is returned once.
@BuiltValue()
abstract class WebhookSubscriptionCreate
    implements
        Built<WebhookSubscriptionCreate, WebhookSubscriptionCreateBuilder> {
  /// HTTPS URL; the host must be in `webhook.allowed_hosts`.
  @BuiltValueField(wireName: r'url')
  String get url;

  @BuiltValueField(wireName: r'event_types')
  BuiltList<String> get eventTypes;

  /// Also sign with Standard Webhooks `v1` (HMAC); the secret is returned once.
  @BuiltValueField(wireName: r'enable_hmac')
  bool? get enableHmac;

  WebhookSubscriptionCreate._();

  factory WebhookSubscriptionCreate([
    void updates(WebhookSubscriptionCreateBuilder b),
  ]) = _$WebhookSubscriptionCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookSubscriptionCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookSubscriptionCreate> get serializer =>
      _$WebhookSubscriptionCreateSerializer();
}

class _$WebhookSubscriptionCreateSerializer
    implements PrimitiveSerializer<WebhookSubscriptionCreate> {
  @override
  final Iterable<Type> types = const [
    WebhookSubscriptionCreate,
    _$WebhookSubscriptionCreate,
  ];

  @override
  final String wireName = r'WebhookSubscriptionCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookSubscriptionCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'url';
    yield serializers.serialize(
      object.url,
      specifiedType: const FullType(String),
    );
    yield r'event_types';
    yield serializers.serialize(
      object.eventTypes,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    if (object.enableHmac != null) {
      yield r'enable_hmac';
      yield serializers.serialize(
        object.enableHmac,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionCreate object, {
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
    required WebhookSubscriptionCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.url = valueDes;
          break;
        case r'event_types':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.eventTypes.replace(valueDes);
          break;
        case r'enable_hmac':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.enableHmac = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookSubscriptionCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookSubscriptionCreateBuilder();
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
