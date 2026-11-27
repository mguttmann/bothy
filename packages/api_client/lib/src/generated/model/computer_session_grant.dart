//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_session_grant.g.dart';

/// Everything `bothy-computer-mcp` needs to attach and run the Noise handshake (27 5.1 step 6). The token is valid once and for `computer.attach_token_ttl`.
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [token] - Compact JWS, `typ = computer-session+jwt`, `alg = EdDSA`, with the claims of 27 5.2.
/// * [attachPath] - Path of the attach connection (`computer.attach`).
/// * [noisePublicKey] - The static Noise public key of the host that `bothy-computer-mcp` checks in the handshake (27 10.4).
/// * [keyFingerprint] - Fingerprint of `noise_public_key`.
/// * [keyGeneration] - Generation of the host key.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class ComputerSessionGrant
    implements Built<ComputerSessionGrant, ComputerSessionGrantBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  /// Compact JWS, `typ = computer-session+jwt`, `alg = EdDSA`, with the claims of 27 5.2.
  @BuiltValueField(wireName: r'token')
  String get token;

  /// Path of the attach connection (`computer.attach`).
  @BuiltValueField(wireName: r'attach_path')
  String get attachPath;

  /// The static Noise public key of the host that `bothy-computer-mcp` checks in the handshake (27 10.4).
  @BuiltValueField(wireName: r'noise_public_key')
  String get noisePublicKey;

  /// Fingerprint of `noise_public_key`.
  @BuiltValueField(wireName: r'key_fingerprint')
  String get keyFingerprint;

  /// Generation of the host key.
  @BuiltValueField(wireName: r'key_generation')
  int get keyGeneration;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  ComputerSessionGrant._();

  factory ComputerSessionGrant([void updates(ComputerSessionGrantBuilder b)]) =
      _$ComputerSessionGrant;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerSessionGrantBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerSessionGrant> get serializer =>
      _$ComputerSessionGrantSerializer();
}

class _$ComputerSessionGrantSerializer
    implements PrimitiveSerializer<ComputerSessionGrant> {
  @override
  final Iterable<Type> types = const [
    ComputerSessionGrant,
    _$ComputerSessionGrant,
  ];

  @override
  final String wireName = r'ComputerSessionGrant';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerSessionGrant object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'token';
    yield serializers.serialize(
      object.token,
      specifiedType: const FullType(String),
    );
    yield r'attach_path';
    yield serializers.serialize(
      object.attachPath,
      specifiedType: const FullType(String),
    );
    yield r'noise_public_key';
    yield serializers.serialize(
      object.noisePublicKey,
      specifiedType: const FullType(String),
    );
    yield r'key_fingerprint';
    yield serializers.serialize(
      object.keyFingerprint,
      specifiedType: const FullType(String),
    );
    yield r'key_generation';
    yield serializers.serialize(
      object.keyGeneration,
      specifiedType: const FullType(int),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerSessionGrant object, {
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
    required ComputerSessionGrantBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        case r'token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.token = valueDes;
          break;
        case r'attach_path':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.attachPath = valueDes;
          break;
        case r'noise_public_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.noisePublicKey = valueDes;
          break;
        case r'key_fingerprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.keyFingerprint = valueDes;
          break;
        case r'key_generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.keyGeneration = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComputerSessionGrant deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerSessionGrantBuilder();
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
