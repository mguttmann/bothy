//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_resumed.g.dart';

/// GatewayResumed
///
/// Properties:
/// * [gatewaySessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [headSeq]
/// * [syncEpoch] - Time line of the event log; a client stores it with its cursor (04, 7.5).
@BuiltValue()
abstract class GatewayResumed
    implements Built<GatewayResumed, GatewayResumedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'gateway_session_id')
  String get gatewaySessionId;

  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  /// Time line of the event log; a client stores it with its cursor (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  GatewayResumed._();

  factory GatewayResumed([void updates(GatewayResumedBuilder b)]) =
      _$GatewayResumed;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayResumedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayResumed> get serializer =>
      _$GatewayResumedSerializer();
}

class _$GatewayResumedSerializer
    implements PrimitiveSerializer<GatewayResumed> {
  @override
  final Iterable<Type> types = const [GatewayResumed, _$GatewayResumed];

  @override
  final String wireName = r'GatewayResumed';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayResumed object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'gateway_session_id';
    yield serializers.serialize(
      object.gatewaySessionId,
      specifiedType: const FullType(String),
    );
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
    yield r'sync_epoch';
    yield serializers.serialize(
      object.syncEpoch,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayResumed object, {
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
    required GatewayResumedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'gateway_session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.gatewaySessionId = valueDes;
          break;
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.syncEpoch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayResumed deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayResumedBuilder();
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
