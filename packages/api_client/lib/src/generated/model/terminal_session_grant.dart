//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/terminal_session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_session_grant.g.dart';

/// Everything the device needs for the Noise handshake (24, 5). The token is valid once and for `terminal.session_token_ttl`.
///
/// Properties:
/// * [session]
/// * [sessionToken] - Compact JWS, `typ = terminal-session+jwt`, `alg = EdDSA`.
/// * [sessionTokenExpiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [streamPath]
/// * [noisePublicKey]
/// * [keyFingerprint]
/// * [keyGeneration]
/// * [noiseProtocol]
/// * [prologueVersion]
@BuiltValue()
abstract class TerminalSessionGrant
    implements Built<TerminalSessionGrant, TerminalSessionGrantBuilder> {
  @BuiltValueField(wireName: r'session')
  TerminalSession get session;

  /// Compact JWS, `typ = terminal-session+jwt`, `alg = EdDSA`.
  @BuiltValueField(wireName: r'session_token')
  String get sessionToken;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'session_token_expires_at')
  DateTime get sessionTokenExpiresAt;

  @BuiltValueField(wireName: r'stream_path')
  String get streamPath;

  @BuiltValueField(wireName: r'noise_public_key')
  String get noisePublicKey;

  @BuiltValueField(wireName: r'key_fingerprint')
  String get keyFingerprint;

  @BuiltValueField(wireName: r'key_generation')
  int get keyGeneration;

  @BuiltValueField(wireName: r'noise_protocol')
  TerminalSessionGrantNoiseProtocolEnum get noiseProtocol;
  // enum noiseProtocolEnum {  Noise_NK_25519_ChaChaPoly_SHA256,  };

  @BuiltValueField(wireName: r'prologue_version')
  TerminalSessionGrantPrologueVersionEnum get prologueVersion;
  // enum prologueVersionEnum {  bothy-terminal/1,  };

  TerminalSessionGrant._();

  factory TerminalSessionGrant([void updates(TerminalSessionGrantBuilder b)]) =
      _$TerminalSessionGrant;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalSessionGrantBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalSessionGrant> get serializer =>
      _$TerminalSessionGrantSerializer();
}

class _$TerminalSessionGrantSerializer
    implements PrimitiveSerializer<TerminalSessionGrant> {
  @override
  final Iterable<Type> types = const [
    TerminalSessionGrant,
    _$TerminalSessionGrant,
  ];

  @override
  final String wireName = r'TerminalSessionGrant';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalSessionGrant object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session';
    yield serializers.serialize(
      object.session,
      specifiedType: const FullType(TerminalSession),
    );
    yield r'session_token';
    yield serializers.serialize(
      object.sessionToken,
      specifiedType: const FullType(String),
    );
    yield r'session_token_expires_at';
    yield serializers.serialize(
      object.sessionTokenExpiresAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'stream_path';
    yield serializers.serialize(
      object.streamPath,
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
    yield r'noise_protocol';
    yield serializers.serialize(
      object.noiseProtocol,
      specifiedType: const FullType(TerminalSessionGrantNoiseProtocolEnum),
    );
    yield r'prologue_version';
    yield serializers.serialize(
      object.prologueVersion,
      specifiedType: const FullType(TerminalSessionGrantPrologueVersionEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TerminalSessionGrant object, {
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
    required TerminalSessionGrantBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TerminalSession),
          ) as TerminalSession;
          result.session.replace(valueDes);
          break;
        case r'session_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionToken = valueDes;
          break;
        case r'session_token_expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.sessionTokenExpiresAt = valueDes;
          break;
        case r'stream_path':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.streamPath = valueDes;
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
        case r'noise_protocol':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              TerminalSessionGrantNoiseProtocolEnum,
            ),
          ) as TerminalSessionGrantNoiseProtocolEnum;
          result.noiseProtocol = valueDes;
          break;
        case r'prologue_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              TerminalSessionGrantPrologueVersionEnum,
            ),
          ) as TerminalSessionGrantPrologueVersionEnum;
          result.prologueVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TerminalSessionGrant deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalSessionGrantBuilder();
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

class TerminalSessionGrantNoiseProtocolEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'Noise_NK_25519_ChaChaPoly_SHA256')
  static const TerminalSessionGrantNoiseProtocolEnum
  noiseNK25519ChaChaPolySHA256 =
      _$terminalSessionGrantNoiseProtocolEnum_noiseNK25519ChaChaPolySHA256;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TerminalSessionGrantNoiseProtocolEnum unknownDefaultOpenApi =
      _$terminalSessionGrantNoiseProtocolEnum_unknownDefaultOpenApi;

  static Serializer<TerminalSessionGrantNoiseProtocolEnum> get serializer =>
      _$terminalSessionGrantNoiseProtocolEnumSerializer;

  const TerminalSessionGrantNoiseProtocolEnum._(String name) : super(name);

  static BuiltSet<TerminalSessionGrantNoiseProtocolEnum> get values =>
      _$terminalSessionGrantNoiseProtocolEnumValues;
  static TerminalSessionGrantNoiseProtocolEnum valueOf(String name) =>
      _$terminalSessionGrantNoiseProtocolEnumValueOf(name);
}

class TerminalSessionGrantPrologueVersionEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'bothy-terminal/1')
  static const TerminalSessionGrantPrologueVersionEnum bothyTerminalSlash1 =
      _$terminalSessionGrantPrologueVersionEnum_bothyTerminalSlash1;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TerminalSessionGrantPrologueVersionEnum unknownDefaultOpenApi =
      _$terminalSessionGrantPrologueVersionEnum_unknownDefaultOpenApi;

  static Serializer<TerminalSessionGrantPrologueVersionEnum> get serializer =>
      _$terminalSessionGrantPrologueVersionEnumSerializer;

  const TerminalSessionGrantPrologueVersionEnum._(String name) : super(name);

  static BuiltSet<TerminalSessionGrantPrologueVersionEnum> get values =>
      _$terminalSessionGrantPrologueVersionEnumValues;
  static TerminalSessionGrantPrologueVersionEnum valueOf(String name) =>
      _$terminalSessionGrantPrologueVersionEnumValueOf(name);
}
