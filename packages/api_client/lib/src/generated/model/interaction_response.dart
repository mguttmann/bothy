//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'interaction_response.g.dart';

/// InteractionResponse
///
/// Properties:
/// * [text]
@BuiltValue()
abstract class InteractionResponse
    implements Built<InteractionResponse, InteractionResponseBuilder> {
  @BuiltValueField(wireName: r'text')
  String? get text;

  InteractionResponse._();

  factory InteractionResponse([void updates(InteractionResponseBuilder b)]) =
      _$InteractionResponse;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InteractionResponseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InteractionResponse> get serializer =>
      _$InteractionResponseSerializer();
}

class _$InteractionResponseSerializer
    implements PrimitiveSerializer<InteractionResponse> {
  @override
  final Iterable<Type> types = const [
    InteractionResponse,
    _$InteractionResponse,
  ];

  @override
  final String wireName = r'InteractionResponse';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InteractionResponse object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.text != null) {
      yield r'text';
      yield serializers.serialize(
        object.text,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    InteractionResponse object, {
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
    required InteractionResponseBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.text = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InteractionResponse deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InteractionResponseBuilder();
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
