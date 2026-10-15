//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_key.g.dart';

/// WebhookKey
///
/// Properties:
/// * [kid]
/// * [publicKey] - Ed25519 public key, base64, with prefix `whpk_`.
/// * [state]
/// * [notAfter] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class WebhookKey implements Built<WebhookKey, WebhookKeyBuilder> {
  @BuiltValueField(wireName: r'kid')
  String get kid;

  /// Ed25519 public key, base64, with prefix `whpk_`.
  @BuiltValueField(wireName: r'public_key')
  String get publicKey;

  @BuiltValueField(wireName: r'state')
  WebhookKeyStateEnum get state;
  // enum stateEnum {  next,  active,  retiring,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'not_after')
  DateTime? get notAfter;

  WebhookKey._();

  factory WebhookKey([void updates(WebhookKeyBuilder b)]) = _$WebhookKey;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebhookKeyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebhookKey> get serializer => _$WebhookKeySerializer();
}

class _$WebhookKeySerializer implements PrimitiveSerializer<WebhookKey> {
  @override
  final Iterable<Type> types = const [WebhookKey, _$WebhookKey];

  @override
  final String wireName = r'WebhookKey';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebhookKey object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kid';
    yield serializers.serialize(
      object.kid,
      specifiedType: const FullType(String),
    );
    yield r'public_key';
    yield serializers.serialize(
      object.publicKey,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(WebhookKeyStateEnum),
    );
    if (object.notAfter != null) {
      yield r'not_after';
      yield serializers.serialize(
        object.notAfter,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    WebhookKey object, {
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
    required WebhookKeyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.kid = valueDes;
          break;
        case r'public_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.publicKey = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(WebhookKeyStateEnum),
          ) as WebhookKeyStateEnum;
          result.state = valueDes;
          break;
        case r'not_after':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.notAfter = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  WebhookKey deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebhookKeyBuilder();
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

class WebhookKeyStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'next')
  static const WebhookKeyStateEnum next = _$webhookKeyStateEnum_next;
  @BuiltValueEnumConst(wireName: r'active')
  static const WebhookKeyStateEnum active = _$webhookKeyStateEnum_active;
  @BuiltValueEnumConst(wireName: r'retiring')
  static const WebhookKeyStateEnum retiring = _$webhookKeyStateEnum_retiring;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WebhookKeyStateEnum unknownDefaultOpenApi =
      _$webhookKeyStateEnum_unknownDefaultOpenApi;

  static Serializer<WebhookKeyStateEnum> get serializer =>
      _$webhookKeyStateEnumSerializer;

  const WebhookKeyStateEnum._(String name) : super(name);

  static BuiltSet<WebhookKeyStateEnum> get values =>
      _$webhookKeyStateEnumValues;
  static WebhookKeyStateEnum valueOf(String name) =>
      _$webhookKeyStateEnumValueOf(name);
}
