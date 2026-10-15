//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pairing_complete_request.g.dart';

/// PairingCompleteRequest
///
/// Properties:
/// * [pairingId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class PairingCompleteRequest
    implements Built<PairingCompleteRequest, PairingCompleteRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'pairing_id')
  String get pairingId;

  PairingCompleteRequest._();

  factory PairingCompleteRequest([
    void updates(PairingCompleteRequestBuilder b),
  ]) = _$PairingCompleteRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PairingCompleteRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PairingCompleteRequest> get serializer =>
      _$PairingCompleteRequestSerializer();
}

class _$PairingCompleteRequestSerializer
    implements PrimitiveSerializer<PairingCompleteRequest> {
  @override
  final Iterable<Type> types = const [
    PairingCompleteRequest,
    _$PairingCompleteRequest,
  ];

  @override
  final String wireName = r'PairingCompleteRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PairingCompleteRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pairing_id';
    yield serializers.serialize(
      object.pairingId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PairingCompleteRequest object, {
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
    required PairingCompleteRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pairing_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pairingId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PairingCompleteRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PairingCompleteRequestBuilder();
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
