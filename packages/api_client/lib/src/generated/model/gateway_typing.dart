//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_typing.g.dart';

/// GatewayTyping
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class GatewayTyping
    implements Built<GatewayTyping, GatewayTypingBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  GatewayTyping._();

  factory GatewayTyping([void updates(GatewayTypingBuilder b)]) =
      _$GatewayTyping;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayTypingBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayTyping> get serializer =>
      _$GatewayTypingSerializer();
}

class _$GatewayTypingSerializer implements PrimitiveSerializer<GatewayTyping> {
  @override
  final Iterable<Type> types = const [GatewayTyping, _$GatewayTyping];

  @override
  final String wireName = r'GatewayTyping';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayTyping object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayTyping object, {
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
    required GatewayTypingBuilder result,
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayTyping deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayTypingBuilder();
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
