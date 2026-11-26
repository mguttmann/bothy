//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_session_open.g.dart';

/// ComputerSessionOpen
///
/// Properties:
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [clientEphemeralKeyHash] - SHA-256 of the ephemeral X25519 public key that `bothy-computer-mcp` sends in Noise message 1, base64url without padding; becomes the claim `cek`.
/// * [pinChanged] - `true` when `bothy-computer-mcp` replaced its pin of the host key after a re-pairing (27 10.4); recorded in the audit entry of `computer.open`.
@BuiltValue()
abstract class ComputerSessionOpen
    implements Built<ComputerSessionOpen, ComputerSessionOpenBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  /// SHA-256 of the ephemeral X25519 public key that `bothy-computer-mcp` sends in Noise message 1, base64url without padding; becomes the claim `cek`.
  @BuiltValueField(wireName: r'client_ephemeral_key_hash')
  String get clientEphemeralKeyHash;

  /// `true` when `bothy-computer-mcp` replaced its pin of the host key after a re-pairing (27 10.4); recorded in the audit entry of `computer.open`.
  @BuiltValueField(wireName: r'pin_changed')
  bool? get pinChanged;

  ComputerSessionOpen._();

  factory ComputerSessionOpen([void updates(ComputerSessionOpenBuilder b)]) =
      _$ComputerSessionOpen;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerSessionOpenBuilder b) => b..pinChanged = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerSessionOpen> get serializer =>
      _$ComputerSessionOpenSerializer();
}

class _$ComputerSessionOpenSerializer
    implements PrimitiveSerializer<ComputerSessionOpen> {
  @override
  final Iterable<Type> types = const [
    ComputerSessionOpen,
    _$ComputerSessionOpen,
  ];

  @override
  final String wireName = r'ComputerSessionOpen';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerSessionOpen object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'client_ephemeral_key_hash';
    yield serializers.serialize(
      object.clientEphemeralKeyHash,
      specifiedType: const FullType(String),
    );
    if (object.pinChanged != null) {
      yield r'pin_changed';
      yield serializers.serialize(
        object.pinChanged,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerSessionOpen object, {
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
    required ComputerSessionOpenBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostId = valueDes;
          break;
        case r'client_ephemeral_key_hash':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientEphemeralKeyHash = valueDes;
          break;
        case r'pin_changed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.pinChanged = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComputerSessionOpen deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerSessionOpenBuilder();
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
