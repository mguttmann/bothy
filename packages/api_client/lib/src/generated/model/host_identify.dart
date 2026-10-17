//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_identify.g.dart';

/// Login of a host service. `proof` is a compact JWS (`alg = EdDSA`, `typ = host-auth+jwt`) over `{challenge, host_id, iat}` with the registered Ed25519 key.
///
/// Properties:
/// * [protocolVersion]
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [proof]
/// * [hostVersion]
/// * [sessionId] - Only on a data connection; the session the connection belongs to.
/// * [liveSessions] - Only on the control connection, always present there (empty when none). The sessions the host service knows, those whose PTY still runs and those whose end it could not report yet (it reports that end with `session.ended` after `ready`). After a restart of the host service the list is empty. The server ends every other session of the host in state `active` or `detached` with `host_restarted` (24, 6.2).
@BuiltValue()
abstract class HostIdentify
    implements Built<HostIdentify, HostIdentifyBuilder> {
  @BuiltValueField(wireName: r'protocol_version')
  int get protocolVersion;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  @BuiltValueField(wireName: r'proof')
  String get proof;

  @BuiltValueField(wireName: r'host_version')
  String get hostVersion;

  /// Only on a data connection; the session the connection belongs to.
  @BuiltValueField(wireName: r'session_id')
  String? get sessionId;

  /// Only on the control connection, always present there (empty when none). The sessions the host service knows, those whose PTY still runs and those whose end it could not report yet (it reports that end with `session.ended` after `ready`). After a restart of the host service the list is empty. The server ends every other session of the host in state `active` or `detached` with `host_restarted` (24, 6.2).
  @BuiltValueField(wireName: r'live_sessions')
  BuiltList<String>? get liveSessions;

  HostIdentify._();

  factory HostIdentify([void updates(HostIdentifyBuilder b)]) = _$HostIdentify;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostIdentifyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostIdentify> get serializer => _$HostIdentifySerializer();
}

class _$HostIdentifySerializer implements PrimitiveSerializer<HostIdentify> {
  @override
  final Iterable<Type> types = const [HostIdentify, _$HostIdentify];

  @override
  final String wireName = r'HostIdentify';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostIdentify object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_version';
    yield serializers.serialize(
      object.protocolVersion,
      specifiedType: const FullType(int),
    );
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'proof';
    yield serializers.serialize(
      object.proof,
      specifiedType: const FullType(String),
    );
    yield r'host_version';
    yield serializers.serialize(
      object.hostVersion,
      specifiedType: const FullType(String),
    );
    if (object.sessionId != null) {
      yield r'session_id';
      yield serializers.serialize(
        object.sessionId,
        specifiedType: const FullType(String),
      );
    }
    if (object.liveSessions != null) {
      yield r'live_sessions';
      yield serializers.serialize(
        object.liveSessions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostIdentify object, {
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
    required HostIdentifyBuilder result,
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
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostId = valueDes;
          break;
        case r'proof':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.proof = valueDes;
          break;
        case r'host_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostVersion = valueDes;
          break;
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sessionId = valueDes;
          break;
        case r'live_sessions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.liveSessions.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostIdentify deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostIdentifyBuilder();
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
