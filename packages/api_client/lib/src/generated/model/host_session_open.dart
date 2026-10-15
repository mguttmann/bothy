//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_session_open.g.dart';

/// The server asks the host service to open the data connection of a session.
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [dataPath]
/// * [profile]
/// * [expiresAt] - Without a valid handshake until then the host service drops the request (`terminal.open_timeout`).
@BuiltValue()
abstract class HostSessionOpen
    implements Built<HostSessionOpen, HostSessionOpenBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'data_path')
  String get dataPath;

  @BuiltValueField(wireName: r'profile')
  String get profile;

  /// Without a valid handshake until then the host service drops the request (`terminal.open_timeout`).
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  HostSessionOpen._();

  factory HostSessionOpen([void updates(HostSessionOpenBuilder b)]) =
      _$HostSessionOpen;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSessionOpenBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSessionOpen> get serializer =>
      _$HostSessionOpenSerializer();
}

class _$HostSessionOpenSerializer
    implements PrimitiveSerializer<HostSessionOpen> {
  @override
  final Iterable<Type> types = const [HostSessionOpen, _$HostSessionOpen];

  @override
  final String wireName = r'HostSessionOpen';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSessionOpen object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'data_path';
    yield serializers.serialize(
      object.dataPath,
      specifiedType: const FullType(String),
    );
    yield r'profile';
    yield serializers.serialize(
      object.profile,
      specifiedType: const FullType(String),
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
    HostSessionOpen object, {
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
    required HostSessionOpenBuilder result,
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
        case r'data_path':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dataPath = valueDes;
          break;
        case r'profile':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.profile = valueDes;
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
  HostSessionOpen deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSessionOpenBuilder();
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
