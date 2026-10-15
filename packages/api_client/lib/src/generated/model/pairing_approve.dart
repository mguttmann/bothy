//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pairing_approve.g.dart';

/// PairingApprove
///
/// Properties:
/// * [pairingCode]
@BuiltValue()
abstract class PairingApprove
    implements Built<PairingApprove, PairingApproveBuilder> {
  @BuiltValueField(wireName: r'pairing_code')
  String get pairingCode;

  PairingApprove._();

  factory PairingApprove([void updates(PairingApproveBuilder b)]) =
      _$PairingApprove;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PairingApproveBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PairingApprove> get serializer =>
      _$PairingApproveSerializer();
}

class _$PairingApproveSerializer
    implements PrimitiveSerializer<PairingApprove> {
  @override
  final Iterable<Type> types = const [PairingApprove, _$PairingApprove];

  @override
  final String wireName = r'PairingApprove';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PairingApprove object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pairing_code';
    yield serializers.serialize(
      object.pairingCode,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PairingApprove object, {
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
    required PairingApproveBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pairing_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pairingCode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PairingApprove deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PairingApproveBuilder();
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
