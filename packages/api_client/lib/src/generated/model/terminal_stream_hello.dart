//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_stream_hello.g.dart';

/// First text frame of the device stream; the binary Noise frames follow `identify`.
///
/// Properties:
/// * [protocolVersions]
/// * [dpopNonce]
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class TerminalStreamHello
    implements Built<TerminalStreamHello, TerminalStreamHelloBuilder> {
  @BuiltValueField(wireName: r'protocol_versions')
  BuiltList<int> get protocolVersions;

  @BuiltValueField(wireName: r'dpop_nonce')
  String get dpopNonce;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime get serverTime;

  TerminalStreamHello._();

  factory TerminalStreamHello([void updates(TerminalStreamHelloBuilder b)]) =
      _$TerminalStreamHello;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalStreamHelloBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalStreamHello> get serializer =>
      _$TerminalStreamHelloSerializer();
}

class _$TerminalStreamHelloSerializer
    implements PrimitiveSerializer<TerminalStreamHello> {
  @override
  final Iterable<Type> types = const [
    TerminalStreamHello,
    _$TerminalStreamHello,
  ];

  @override
  final String wireName = r'TerminalStreamHello';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalStreamHello object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_versions';
    yield serializers.serialize(
      object.protocolVersions,
      specifiedType: const FullType(BuiltList, [FullType(int)]),
    );
    yield r'dpop_nonce';
    yield serializers.serialize(
      object.dpopNonce,
      specifiedType: const FullType(String),
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
    TerminalStreamHello object, {
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
    required TerminalStreamHelloBuilder result,
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
        case r'dpop_nonce':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dpopNonce = valueDes;
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
  TerminalStreamHello deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalStreamHelloBuilder();
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
