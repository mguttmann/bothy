//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_subscription_update.g.dart';

/// WebhookSubscriptionUpdate
///
/// Properties:
/// * [url]
/// * [eventTypes]
/// * [state]
/// * [rotateHmacSecret]
@BuiltValue()
abstract class WebhookSubscriptionUpdate
    implements
        Built<WebhookSubscriptionUpdate, WebhookSubscriptionUpdateBuilder> {
  @BuiltValueField(wireName: r'url')
  String? get url;

  @BuiltValueField(wireName: r'event_types')
  BuiltList<String>? get eventTypes;

  @BuiltValueField(wireName: r'state')
  WebhookSubscriptionUpdateStateEnum? get state;
  // enum stateEnum {  active,  paused,  };

  @BuiltValueField(wireName: r'rotate_hmac_secret')
  bool? get rotateHmacSecret;

  WebhookSubscriptionUpdate._();

  factory WebhookSubscriptionUpdate([
    void updates(WebhookSubscriptionUpdateBuilder b),
  ]) = _$WebhookSubscriptionUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookSubscriptionUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookSubscriptionUpdate> get serializer =>
      _$WebhookSubscriptionUpdateSerializer();
}

class _$WebhookSubscriptionUpdateSerializer
    implements PrimitiveSerializer<WebhookSubscriptionUpdate> {
  @override
  final Iterable<Type> types = const [
    WebhookSubscriptionUpdate,
    _$WebhookSubscriptionUpdate,
  ];

  @override
  final String wireName = r'WebhookSubscriptionUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookSubscriptionUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.url != null) {
      yield r'url';
      yield serializers.serialize(
        object.url,
        specifiedType: const FullType(String),
      );
    }
    if (object.eventTypes != null) {
      yield r'event_types';
      yield serializers.serialize(
        object.eventTypes,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.state != null) {
      yield r'state';
      yield serializers.serialize(
        object.state,
        specifiedType: const FullType(WebhookSubscriptionUpdateStateEnum),
      );
    }
    if (object.rotateHmacSecret != null) {
      yield r'rotate_hmac_secret';
      yield serializers.serialize(
        object.rotateHmacSecret,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionUpdate object, {
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
    required WebhookSubscriptionUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.url = valueDes;
          break;
        case r'event_types':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.eventTypes.replace(valueDes);
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              WebhookSubscriptionUpdateStateEnum,
            ),
          ) as WebhookSubscriptionUpdateStateEnum?;
          if (valueDes == null) continue;
          result.state = valueDes;
          break;
        case r'rotate_hmac_secret':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.rotateHmacSecret = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookSubscriptionUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookSubscriptionUpdateBuilder();
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

class WebhookSubscriptionUpdateStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const WebhookSubscriptionUpdateStateEnum active =
      _$webhookSubscriptionUpdateStateEnum_active;
  @BuiltValueEnumConst(wireName: r'paused')
  static const WebhookSubscriptionUpdateStateEnum paused =
      _$webhookSubscriptionUpdateStateEnum_paused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WebhookSubscriptionUpdateStateEnum unknownDefaultOpenApi =
      _$webhookSubscriptionUpdateStateEnum_unknownDefaultOpenApi;

  static Serializer<WebhookSubscriptionUpdateStateEnum> get serializer =>
      _$webhookSubscriptionUpdateStateEnumSerializer;

  const WebhookSubscriptionUpdateStateEnum._(String name) : super(name);

  static BuiltSet<WebhookSubscriptionUpdateStateEnum> get values =>
      _$webhookSubscriptionUpdateStateEnumValues;
  static WebhookSubscriptionUpdateStateEnum valueOf(String name) =>
      _$webhookSubscriptionUpdateStateEnumValueOf(name);
}
