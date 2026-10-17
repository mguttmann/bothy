//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/gdpr_request.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_request_payload.g.dart';

/// GdprRequestPayload
///
/// Properties:
/// * [request]
@BuiltValue()
abstract class GdprRequestPayload
    implements Built<GdprRequestPayload, GdprRequestPayloadBuilder> {
  @BuiltValueField(wireName: r'request')
  GdprRequest get request;

  GdprRequestPayload._();

  factory GdprRequestPayload([void updates(GdprRequestPayloadBuilder b)]) =
      _$GdprRequestPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GdprRequestPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GdprRequestPayload> get serializer =>
      _$GdprRequestPayloadSerializer();
}

class _$GdprRequestPayloadSerializer
    implements PrimitiveSerializer<GdprRequestPayload> {
  @override
  final Iterable<Type> types = const [GdprRequestPayload, _$GdprRequestPayload];

  @override
  final String wireName = r'GdprRequestPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GdprRequestPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'request';
    yield serializers.serialize(
      object.request,
      specifiedType: const FullType(GdprRequest),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GdprRequestPayload object, {
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
    required GdprRequestPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'request':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GdprRequest),
          ) as GdprRequest;
          result.request.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GdprRequestPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GdprRequestPayloadBuilder();
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
