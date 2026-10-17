//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device_approve.g.dart';

/// DeviceApprove
///
/// Properties:
/// * [verificationCode] - Code shown on the pending device, typed by the approving person.
@BuiltValue()
abstract class DeviceApprove
    implements Built<DeviceApprove, DeviceApproveBuilder> {
  /// Code shown on the pending device, typed by the approving person.
  @BuiltValueField(wireName: r'verification_code')
  String get verificationCode;

  DeviceApprove._();

  factory DeviceApprove([void updates(DeviceApproveBuilder b)]) =
      _$DeviceApprove;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeviceApproveBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DeviceApprove> get serializer =>
      _$DeviceApproveSerializer();
}

class _$DeviceApproveSerializer implements PrimitiveSerializer<DeviceApprove> {
  @override
  final Iterable<Type> types = const [DeviceApprove, _$DeviceApprove];

  @override
  final String wireName = r'DeviceApprove';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DeviceApprove object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'verification_code';
    yield serializers.serialize(
      object.verificationCode,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DeviceApprove object, {
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
    required DeviceApproveBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'verification_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.verificationCode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DeviceApprove deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeviceApproveBuilder();
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
