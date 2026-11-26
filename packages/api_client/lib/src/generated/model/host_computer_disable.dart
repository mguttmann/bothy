//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_disable.g.dart';

/// HostComputerDisable
///
/// Properties:
/// * [generation]
@BuiltValue()
abstract class HostComputerDisable
    implements Built<HostComputerDisable, HostComputerDisableBuilder> {
  @BuiltValueField(wireName: r'generation')
  int get generation;

  HostComputerDisable._();

  factory HostComputerDisable([void updates(HostComputerDisableBuilder b)]) =
      _$HostComputerDisable;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerDisableBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerDisable> get serializer =>
      _$HostComputerDisableSerializer();
}

class _$HostComputerDisableSerializer
    implements PrimitiveSerializer<HostComputerDisable> {
  @override
  final Iterable<Type> types = const [
    HostComputerDisable,
    _$HostComputerDisable,
  ];

  @override
  final String wireName = r'HostComputerDisable';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerDisable object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'generation';
    yield serializers.serialize(
      object.generation,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerDisable object, {
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
    required HostComputerDisableBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.generation = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerDisable deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerDisableBuilder();
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
