//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_stream_identify.g.dart';

/// Native apps send an access token with DPoP proof (`htm = GET`, `htu` = HTTPS form of the stream URL, `nonce` from `hello`, `ath`). The web console sends neither: its stream authenticates with the console session cookie of the upgrade request and the same-origin check of the upgrade (docs/spec/08-identitaet.md section 7.5, docs/spec/15-client-architektur.md section 23.2); the server rejects an identify without token from any other session. Only the device of the current session token passes (`terminal.stream`).
///
/// Properties:
/// * [protocolVersion]
/// * [accessToken]
/// * [dpop]
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class TerminalStreamIdentify
    implements Built<TerminalStreamIdentify, TerminalStreamIdentifyBuilder> {
  @BuiltValueField(wireName: r'protocol_version')
  int get protocolVersion;

  @BuiltValueField(wireName: r'access_token')
  String? get accessToken;

  @BuiltValueField(wireName: r'dpop')
  String? get dpop;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  TerminalStreamIdentify._();

  factory TerminalStreamIdentify([
    void updates(TerminalStreamIdentifyBuilder b),
  ]) = _$TerminalStreamIdentify;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalStreamIdentifyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalStreamIdentify> get serializer =>
      _$TerminalStreamIdentifySerializer();
}

class _$TerminalStreamIdentifySerializer
    implements PrimitiveSerializer<TerminalStreamIdentify> {
  @override
  final Iterable<Type> types = const [
    TerminalStreamIdentify,
    _$TerminalStreamIdentify,
  ];

  @override
  final String wireName = r'TerminalStreamIdentify';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalStreamIdentify object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'protocol_version';
    yield serializers.serialize(
      object.protocolVersion,
      specifiedType: const FullType(int),
    );
    if (object.accessToken != null) {
      yield r'access_token';
      yield serializers.serialize(
        object.accessToken,
        specifiedType: const FullType(String),
      );
    }
    if (object.dpop != null) {
      yield r'dpop';
      yield serializers.serialize(
        object.dpop,
        specifiedType: const FullType(String),
      );
    }
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TerminalStreamIdentify object, {
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
    required TerminalStreamIdentifyBuilder result,
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
        case r'access_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.accessToken = valueDes;
          break;
        case r'dpop':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dpop = valueDes;
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TerminalStreamIdentify deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalStreamIdentifyBuilder();
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
