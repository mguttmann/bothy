//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/webhook_key.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_keys.g.dart';

/// Public keys for Standard Webhooks `v1a` signatures.
///
/// Properties:
/// * [keys]
@BuiltValue()
abstract class WebhookKeys implements Built<WebhookKeys, WebhookKeysBuilder> {
  @BuiltValueField(wireName: r'keys')
  BuiltList<WebhookKey> get keys;

  WebhookKeys._();

  factory WebhookKeys([void updates(WebhookKeysBuilder b)]) = _$WebhookKeys;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookKeysBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookKeys> get serializer => _$WebhookKeysSerializer();
}

class _$WebhookKeysSerializer implements PrimitiveSerializer<WebhookKeys> {
  @override
  final Iterable<Type> types = const [WebhookKeys, _$WebhookKeys];

  @override
  final String wireName = r'WebhookKeys';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookKeys object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'keys';
    yield serializers.serialize(
      object.keys,
      specifiedType: const FullType(BuiltList, [FullType(WebhookKey)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookKeys object, {
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
    required WebhookKeysBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'keys':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(WebhookKey)]),
          ) as BuiltList<WebhookKey>;
          result.keys.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookKeys deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookKeysBuilder();
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
