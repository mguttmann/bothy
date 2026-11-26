//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host_computer_settings.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_open.g.dart';

/// The server asks the host service to open the computer data connection of a session (27 5.1 step 7); `user_id` is the triggering owner (`sub` of the attach token).
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentName]
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [dataPath]
/// * [expiresAt] - Without both sides paired and the handshake done until then the session ends `open_failed` with `open_timeout` (`computer.open_timeout`).
/// * [settings]
@BuiltValue()
abstract class HostComputerOpen
    implements Built<HostComputerOpen, HostComputerOpenBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'agent_name')
  String get agentName;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  @BuiltValueField(wireName: r'data_path')
  String get dataPath;

  /// Without both sides paired and the handshake done until then the session ends `open_failed` with `open_timeout` (`computer.open_timeout`).
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  @BuiltValueField(wireName: r'settings')
  HostComputerSettings get settings;

  HostComputerOpen._();

  factory HostComputerOpen([void updates(HostComputerOpenBuilder b)]) =
      _$HostComputerOpen;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerOpenBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerOpen> get serializer =>
      _$HostComputerOpenSerializer();
}

class _$HostComputerOpenSerializer
    implements PrimitiveSerializer<HostComputerOpen> {
  @override
  final Iterable<Type> types = const [HostComputerOpen, _$HostComputerOpen];

  @override
  final String wireName = r'HostComputerOpen';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerOpen object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'agent_name';
    yield serializers.serialize(
      object.agentName,
      specifiedType: const FullType(String),
    );
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'data_path';
    yield serializers.serialize(
      object.dataPath,
      specifiedType: const FullType(String),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'settings';
    yield serializers.serialize(
      object.settings,
      specifiedType: const FullType(HostComputerSettings),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerOpen object, {
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
    required HostComputerOpenBuilder result,
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
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'agent_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentName = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
        case r'data_path':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dataPath = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        case r'settings':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerSettings),
          ) as HostComputerSettings;
          result.settings.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerOpen deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerOpenBuilder();
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
