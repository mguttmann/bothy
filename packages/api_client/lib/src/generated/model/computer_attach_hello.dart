//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_attach_hello.g.dart';

/// First text frame of the computer attach connection; `identify` follows, then binary Noise frames only.
///
/// Properties:
/// * [protocolVersions]
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class ComputerAttachHello
    implements Built<ComputerAttachHello, ComputerAttachHelloBuilder> {
  @BuiltValueField(wireName: r'protocol_versions')
  BuiltList<int> get protocolVersions;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime get serverTime;

  ComputerAttachHello._();

  factory ComputerAttachHello([void updates(ComputerAttachHelloBuilder b)]) =
      _$ComputerAttachHello;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerAttachHelloBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerAttachHello> get serializer =>
      _$ComputerAttachHelloSerializer();
}

class _$ComputerAttachHelloSerializer
    implements PrimitiveSerializer<ComputerAttachHello> {
  @override
  final Iterable<Type> types = const [
    ComputerAttachHello,
    _$ComputerAttachHello,
  ];

  @override
  final String wireName = r'ComputerAttachHello';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerAttachHello object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_versions';
    yield serializers.serialize(
      object.protocolVersions,
      specifiedType: const FullType(BuiltList, [FullType(int)]),
    );
    yield r'server_time';
    yield serializers.serialize(
      object.serverTime,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerAttachHello object, {
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
    required ComputerAttachHelloBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'protocol_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(int)]),
          ) as BuiltList<int>;
          result.protocolVersions.replace(valueDes);
          break;
        case r'server_time':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.serverTime = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComputerAttachHello deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerAttachHelloBuilder();
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
