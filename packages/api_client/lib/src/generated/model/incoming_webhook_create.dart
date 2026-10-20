//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_create.g.dart';

/// Body of `createIncomingWebhook`. `app_id` names an app principal whose installation is `active` (else `400 validation_failed` with `errors[].code` `app_not_installed`) and that may write in the channel (else `app_cannot_write`); `thread_id`, when present, is a thread of this channel (else `not_in_channel`, an unknown ID too). `name` is not empty and has no control or bidi characters.
///
/// Properties:
/// * [appId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class IncomingWebhookCreate
    implements Built<IncomingWebhookCreate, IncomingWebhookCreateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'app_id')
  String get appId;

  @BuiltValueField(wireName: r'name')
  String get name;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  IncomingWebhookCreate._();

  factory IncomingWebhookCreate([
    void updates(IncomingWebhookCreateBuilder b),
  ]) = _$IncomingWebhookCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookCreate> get serializer =>
      _$IncomingWebhookCreateSerializer();
}

class _$IncomingWebhookCreateSerializer
    implements PrimitiveSerializer<IncomingWebhookCreate> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookCreate,
    _$IncomingWebhookCreate,
  ];

  @override
  final String wireName = r'IncomingWebhookCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'app_id';
    yield serializers.serialize(
      object.appId,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookCreate object, {
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
    required IncomingWebhookCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'app_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appId = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IncomingWebhookCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookCreateBuilder();
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
