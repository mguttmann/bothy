//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/snapshot.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_ready.g.dart';

/// GatewayReady
///
/// Properties:
/// * [gatewaySessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [principal]
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [permissionVersion]
/// * [headSeq]
/// * [snapshot]
/// * [syncEpoch] - Time line of the event log; a client stores it with its cursor (04, 7.5).
@BuiltValue()
abstract class GatewayReady
    implements Built<GatewayReady, GatewayReadyBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'gateway_session_id')
  String get gatewaySessionId;

  @BuiltValueField(wireName: r'principal')
  PrincipalRef get principal;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String? get deviceId;

  @BuiltValueField(wireName: r'permission_version')
  int get permissionVersion;

  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  @BuiltValueField(wireName: r'snapshot')
  Snapshot get snapshot;

  /// Time line of the event log; a client stores it with its cursor (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  GatewayReady._();

  factory GatewayReady([void updates(GatewayReadyBuilder b)]) = _$GatewayReady;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayReadyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayReady> get serializer => _$GatewayReadySerializer();
}

class _$GatewayReadySerializer implements PrimitiveSerializer<GatewayReady> {
  @override
  final Iterable<Type> types = const [GatewayReady, _$GatewayReady];

  @override
  final String wireName = r'GatewayReady';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayReady object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'gateway_session_id';
    yield serializers.serialize(
      object.gatewaySessionId,
      specifiedType: const FullType(String),
    );
    yield r'principal';
    yield serializers.serialize(
      object.principal,
      specifiedType: const FullType(PrincipalRef),
    );
    if (object.deviceId != null) {
      yield r'device_id';
      yield serializers.serialize(
        object.deviceId,
        specifiedType: const FullType(String),
      );
    }
    yield r'permission_version';
    yield serializers.serialize(
      object.permissionVersion,
      specifiedType: const FullType(int),
    );
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
    yield r'snapshot';
    yield serializers.serialize(
      object.snapshot,
      specifiedType: const FullType(Snapshot),
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
    GatewayReady object, {
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
    required GatewayReadyBuilder result,
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
        case r'principal':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.principal.replace(valueDes);
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.deviceId = valueDes;
          break;
        case r'permission_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.permissionVersion = valueDes;
          break;
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        case r'snapshot':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Snapshot),
          ) as Snapshot;
          result.snapshot.replace(valueDes);
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
  GatewayReady deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayReadyBuilder();
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
