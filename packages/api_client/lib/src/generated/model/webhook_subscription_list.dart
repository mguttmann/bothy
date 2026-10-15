//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/webhook_subscription.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_subscription_list.g.dart';

/// WebhookSubscriptionList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class WebhookSubscriptionList
    implements Built<WebhookSubscriptionList, WebhookSubscriptionListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<WebhookSubscription> get items;

  WebhookSubscriptionList._();

  factory WebhookSubscriptionList([
    void updates(WebhookSubscriptionListBuilder b),
  ]) = _$WebhookSubscriptionList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookSubscriptionListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookSubscriptionList> get serializer =>
      _$WebhookSubscriptionListSerializer();
}

class _$WebhookSubscriptionListSerializer
    implements PrimitiveSerializer<WebhookSubscriptionList> {
  @override
  final Iterable<Type> types = const [
    WebhookSubscriptionList,
    _$WebhookSubscriptionList,
  ];

  @override
  final String wireName = r'WebhookSubscriptionList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookSubscriptionList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(WebhookSubscription)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionList object, {
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
    required WebhookSubscriptionListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(WebhookSubscription),
            ]),
          ) as BuiltList<WebhookSubscription>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookSubscriptionList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookSubscriptionListBuilder();
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
