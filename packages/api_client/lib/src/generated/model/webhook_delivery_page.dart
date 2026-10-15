//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/webhook_delivery.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_delivery_page.g.dart';

/// One page. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class WebhookDeliveryPage
    implements Built<WebhookDeliveryPage, WebhookDeliveryPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<WebhookDelivery> get items;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  WebhookDeliveryPage._();

  factory WebhookDeliveryPage([void updates(WebhookDeliveryPageBuilder b)]) =
      _$WebhookDeliveryPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookDeliveryPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookDeliveryPage> get serializer =>
      _$WebhookDeliveryPageSerializer();
}

class _$WebhookDeliveryPageSerializer
    implements PrimitiveSerializer<WebhookDeliveryPage> {
  @override
  final Iterable<Type> types = const [
    WebhookDeliveryPage,
    _$WebhookDeliveryPage,
  ];

  @override
  final String wireName = r'WebhookDeliveryPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookDeliveryPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(WebhookDelivery)]),
    );
    if (object.nextCursor != null) {
      yield r'next_cursor';
      yield serializers.serialize(
        object.nextCursor,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookDeliveryPage object, {
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
    required WebhookDeliveryPageBuilder result,
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
              FullType(WebhookDelivery),
            ]),
          ) as BuiltList<WebhookDelivery>;
          result.items.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.nextCursor = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookDeliveryPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookDeliveryPageBuilder();
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
