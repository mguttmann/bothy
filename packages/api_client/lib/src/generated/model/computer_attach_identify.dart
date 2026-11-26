//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_attach_identify.g.dart';

/// The single-use attach token of the session (27 5.4); the connection carries no agent token.
///
/// Properties:
/// * [protocolVersion]
/// * [token]
@BuiltValue()
abstract class ComputerAttachIdentify
    implements Built<ComputerAttachIdentify, ComputerAttachIdentifyBuilder> {
  @BuiltValueField(wireName: r'protocol_version')
  int get protocolVersion;

  @BuiltValueField(wireName: r'token')
  String get token;

  ComputerAttachIdentify._();

  factory ComputerAttachIdentify([
    void updates(ComputerAttachIdentifyBuilder b),
  ]) = _$ComputerAttachIdentify;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerAttachIdentifyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerAttachIdentify> get serializer =>
      _$ComputerAttachIdentifySerializer();
}

class _$ComputerAttachIdentifySerializer
    implements PrimitiveSerializer<ComputerAttachIdentify> {
  @override
  final Iterable<Type> types = const [
    ComputerAttachIdentify,
    _$ComputerAttachIdentify,
  ];

  @override
  final String wireName = r'ComputerAttachIdentify';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerAttachIdentify object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_version';
    yield serializers.serialize(
      object.protocolVersion,
      specifiedType: const FullType(int),
    );
    yield r'token';
    yield serializers.serialize(
      object.token,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerAttachIdentify object, {
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
    required ComputerAttachIdentifyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'protocol_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.protocolVersion = valueDes;
          break;
        case r'token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.token = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComputerAttachIdentify deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerAttachIdentifyBuilder();
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
