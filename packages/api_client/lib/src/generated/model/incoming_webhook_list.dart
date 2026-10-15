//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/incoming_webhook.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_list.g.dart';

/// IncomingWebhookList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class IncomingWebhookList
    implements Built<IncomingWebhookList, IncomingWebhookListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<IncomingWebhook> get items;

  IncomingWebhookList._();

  factory IncomingWebhookList([void updates(IncomingWebhookListBuilder b)]) =
      _$IncomingWebhookList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookList> get serializer =>
      _$IncomingWebhookListSerializer();
}

class _$IncomingWebhookListSerializer
    implements PrimitiveSerializer<IncomingWebhookList> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookList,
    _$IncomingWebhookList,
  ];

  @override
  final String wireName = r'IncomingWebhookList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(IncomingWebhook)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookList object, {
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
    required IncomingWebhookListBuilder result,
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
              FullType(IncomingWebhook),
            ]),
          ) as BuiltList<IncomingWebhook>;
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
  IncomingWebhookList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookListBuilder();
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
