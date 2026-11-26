//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/computer_close_reason.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_close.g.dart';

/// The server ends a computer session on the host (`computer.close`, 27 12.3).
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
@BuiltValue()
abstract class HostComputerClose
    implements Built<HostComputerClose, HostComputerCloseBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'reason')
  ComputerCloseReason get reason;
  // enum reasonEnum {  done,  stopped_by_user,  device_disabled,  run_ended,  access_revoked,  host_disabled,  host_retired,  host_rekeyed,  host_restarted,  connection_lost,  open_failed,  error,  };

  HostComputerClose._();

  factory HostComputerClose([void updates(HostComputerCloseBuilder b)]) =
      _$HostComputerClose;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerCloseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerClose> get serializer =>
      _$HostComputerCloseSerializer();
}

class _$HostComputerCloseSerializer
    implements PrimitiveSerializer<HostComputerClose> {
  @override
  final Iterable<Type> types = const [HostComputerClose, _$HostComputerClose];

  @override
  final String wireName = r'HostComputerClose';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerClose object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(ComputerCloseReason),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerClose object, {
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
    required HostComputerCloseBuilder result,
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
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ComputerCloseReason),
          ) as ComputerCloseReason;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerClose deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerCloseBuilder();
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
