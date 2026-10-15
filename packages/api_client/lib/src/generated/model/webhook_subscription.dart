//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_subscription.g.dart';

/// WebhookSubscription
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [appId] - Set for subscriptions of an app.
/// * [agentId] - Set for subscriptions of an agent (agent transport, 04 15.6); exactly one of `app_id` and `agent_id`.
/// * [url]
/// * [eventTypes]
/// * [state]
/// * [consecutiveFailures]
/// * [signatureSchemes]
/// * [signingKeyKid]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [disabledAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class WebhookSubscription
    implements Built<WebhookSubscription, WebhookSubscriptionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// Set for subscriptions of an app.
  @BuiltValueField(wireName: r'app_id')
  String? get appId;

  /// Set for subscriptions of an agent (agent transport, 04 15.6); exactly one of `app_id` and `agent_id`.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  @BuiltValueField(wireName: r'url')
  String get url;

  @BuiltValueField(wireName: r'event_types')
  BuiltList<String> get eventTypes;

  @BuiltValueField(wireName: r'state')
  WebhookSubscriptionStateEnum get state;
  // enum stateEnum {  active,  paused,  disabled,  };

  @BuiltValueField(wireName: r'consecutive_failures')
  int get consecutiveFailures;

  @BuiltValueField(wireName: r'signature_schemes')
  BuiltList<WebhookSubscriptionSignatureSchemesEnum> get signatureSchemes;
  // enum signatureSchemesEnum {  v1a,  v1,  };

  @BuiltValueField(wireName: r'signing_key_kid')
  String get signingKeyKid;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'disabled_at')
  DateTime? get disabledAt;

  WebhookSubscription._();

  factory WebhookSubscription([void updates(WebhookSubscriptionBuilder b)]) =
      _$WebhookSubscription;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookSubscriptionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookSubscription> get serializer =>
      _$WebhookSubscriptionSerializer();
}

class _$WebhookSubscriptionSerializer
    implements PrimitiveSerializer<WebhookSubscription> {
  @override
  final Iterable<Type> types = const [
    WebhookSubscription,
    _$WebhookSubscription,
  ];

  @override
  final String wireName = r'WebhookSubscription';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookSubscription object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    if (object.appId != null) {
      yield r'app_id';
      yield serializers.serialize(
        object.appId,
        specifiedType: const FullType(String),
      );
    }
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
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
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(WebhookSubscriptionStateEnum),
    );
    yield r'consecutive_failures';
    yield serializers.serialize(
      object.consecutiveFailures,
      specifiedType: const FullType(int),
    );
    yield r'signature_schemes';
    yield serializers.serialize(
      object.signatureSchemes,
      specifiedType: const FullType(BuiltList, [
        FullType(WebhookSubscriptionSignatureSchemesEnum),
      ]),
    );
    yield r'signing_key_kid';
    yield serializers.serialize(
      object.signingKeyKid,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.disabledAt != null) {
      yield r'disabled_at';
      yield serializers.serialize(
        object.disabledAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscription object, {
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
    required WebhookSubscriptionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'app_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.appId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
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
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(WebhookSubscriptionStateEnum),
          ) as WebhookSubscriptionStateEnum;
          result.state = valueDes;
          break;
        case r'consecutive_failures':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.consecutiveFailures = valueDes;
          break;
        case r'signature_schemes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(WebhookSubscriptionSignatureSchemesEnum),
            ]),
          ) as BuiltList<WebhookSubscriptionSignatureSchemesEnum>;
          result.signatureSchemes.replace(valueDes);
          break;
        case r'signing_key_kid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.signingKeyKid = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'disabled_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.disabledAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookSubscription deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookSubscriptionBuilder();
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

class WebhookSubscriptionStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const WebhookSubscriptionStateEnum active =
      _$webhookSubscriptionStateEnum_active;
  @BuiltValueEnumConst(wireName: r'paused')
  static const WebhookSubscriptionStateEnum paused =
      _$webhookSubscriptionStateEnum_paused;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const WebhookSubscriptionStateEnum disabled =
      _$webhookSubscriptionStateEnum_disabled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WebhookSubscriptionStateEnum unknownDefaultOpenApi =
      _$webhookSubscriptionStateEnum_unknownDefaultOpenApi;

  static Serializer<WebhookSubscriptionStateEnum> get serializer =>
      _$webhookSubscriptionStateEnumSerializer;

  const WebhookSubscriptionStateEnum._(String name) : super(name);

  static BuiltSet<WebhookSubscriptionStateEnum> get values =>
      _$webhookSubscriptionStateEnumValues;
  static WebhookSubscriptionStateEnum valueOf(String name) =>
      _$webhookSubscriptionStateEnumValueOf(name);
}

class WebhookSubscriptionSignatureSchemesEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'v1a')
  static const WebhookSubscriptionSignatureSchemesEnum v1a =
      _$webhookSubscriptionSignatureSchemesEnum_v1a;
  @BuiltValueEnumConst(wireName: r'v1')
  static const WebhookSubscriptionSignatureSchemesEnum v1 =
      _$webhookSubscriptionSignatureSchemesEnum_v1;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WebhookSubscriptionSignatureSchemesEnum unknownDefaultOpenApi =
      _$webhookSubscriptionSignatureSchemesEnum_unknownDefaultOpenApi;

  static Serializer<WebhookSubscriptionSignatureSchemesEnum> get serializer =>
      _$webhookSubscriptionSignatureSchemesEnumSerializer;

  const WebhookSubscriptionSignatureSchemesEnum._(String name) : super(name);

  static BuiltSet<WebhookSubscriptionSignatureSchemesEnum> get values =>
      _$webhookSubscriptionSignatureSchemesEnumValues;
  static WebhookSubscriptionSignatureSchemesEnum valueOf(String name) =>
      _$webhookSubscriptionSignatureSchemesEnumValueOf(name);
}
