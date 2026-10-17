//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'typing_started_payload.g.dart';

/// TypingStartedPayload
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [principalId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class TypingStartedPayload
    implements Built<TypingStartedPayload, TypingStartedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'principal_id')
  String get principalId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  TypingStartedPayload._();

  factory TypingStartedPayload([void updates(TypingStartedPayloadBuilder b)]) =
      _$TypingStartedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TypingStartedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TypingStartedPayload> get serializer =>
      _$TypingStartedPayloadSerializer();
}

class _$TypingStartedPayloadSerializer
    implements PrimitiveSerializer<TypingStartedPayload> {
  @override
  final Iterable<Type> types = const [
    TypingStartedPayload,
    _$TypingStartedPayload,
  ];

  @override
  final String wireName = r'TypingStartedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TypingStartedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'principal_id';
    yield serializers.serialize(
      object.principalId,
      specifiedType: const FullType(String),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TypingStartedPayload object, {
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
    required TypingStartedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'principal_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.principalId = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TypingStartedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TypingStartedPayloadBuilder();
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
