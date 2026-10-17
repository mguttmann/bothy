//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'invariant_violation.g.dart';

/// `i1_violation` names `agent_id`; `i6_violation` names `host_id`.
///
/// Properties:
/// * [code]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [credentialHolderId] - Credential holder of the agent; absent if the agent has none.
/// * [principalIds] - Principals that would gain `trigger` next to the credential holder.
/// * [organizationReady] - True when the agent reports `organization` among its `billing_modes_supported`.
/// * [hint]
@BuiltValue()
abstract class InvariantViolation
    implements Built<InvariantViolation, InvariantViolationBuilder> {
  @BuiltValueField(wireName: r'code')
  String get code;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String? get hostId;

  /// Credential holder of the agent; absent if the agent has none.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  /// Principals that would gain `trigger` next to the credential holder.
  @BuiltValueField(wireName: r'principal_ids')
  BuiltList<String> get principalIds;

  /// True when the agent reports `organization` among its `billing_modes_supported`.
  @BuiltValueField(wireName: r'organization_ready')
  bool? get organizationReady;

  @BuiltValueField(wireName: r'hint')
  String? get hint;

  InvariantViolation._();

  factory InvariantViolation([void updates(InvariantViolationBuilder b)]) =
      _$InvariantViolation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InvariantViolationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InvariantViolation> get serializer =>
      _$InvariantViolationSerializer();
}

class _$InvariantViolationSerializer
    implements PrimitiveSerializer<InvariantViolation> {
  @override
  final Iterable<Type> types = const [InvariantViolation, _$InvariantViolation];

  @override
  final String wireName = r'InvariantViolation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InvariantViolation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'code';
    yield serializers.serialize(
      object.code,
      specifiedType: const FullType(String),
    );
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.hostId != null) {
      yield r'host_id';
      yield serializers.serialize(
        object.hostId,
        specifiedType: const FullType(String),
      );
    }
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    yield r'principal_ids';
    yield serializers.serialize(
      object.principalIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    if (object.organizationReady != null) {
      yield r'organization_ready';
      yield serializers.serialize(
        object.organizationReady,
        specifiedType: const FullType(bool),
      );
    }
    if (object.hint != null) {
      yield r'hint';
      yield serializers.serialize(
        object.hint,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    InvariantViolation object, {
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
    required InvariantViolationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.code = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.hostId = valueDes;
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.credentialHolderId = valueDes;
          break;
        case r'principal_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.principalIds.replace(valueDes);
          break;
        case r'organization_ready':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.organizationReady = valueDes;
          break;
        case r'hint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.hint = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InvariantViolation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InvariantViolationBuilder();
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
