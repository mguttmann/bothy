//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'audit_verify_result.g.dart';

/// AuditVerifyResult
///
/// Properties:
/// * [ok]
/// * [checkedFromSeq]
/// * [checkedToSeq]
/// * [firstBrokenSeq]
/// * [failureReason]
/// * [checkpointsCompared]
/// * [lastCheckpointSeq]
/// * [warnings] - Known value `checkpoint_sink_not_configured`.
@BuiltValue()
abstract class AuditVerifyResult
    implements Built<AuditVerifyResult, AuditVerifyResultBuilder> {
  @BuiltValueField(wireName: r'ok')
  bool get ok;

  @BuiltValueField(wireName: r'checked_from_seq')
  int get checkedFromSeq;

  @BuiltValueField(wireName: r'checked_to_seq')
  int get checkedToSeq;

  @BuiltValueField(wireName: r'first_broken_seq')
  int? get firstBrokenSeq;

  @BuiltValueField(wireName: r'failure_reason')
  AuditVerifyResultFailureReasonEnum? get failureReason;
  // enum failureReasonEnum {  hash_mismatch,  link_mismatch,  seq_gap,  checkpoint_mismatch,  checkpoint_invalid,  };

  @BuiltValueField(wireName: r'checkpoints_compared')
  int get checkpointsCompared;

  @BuiltValueField(wireName: r'last_checkpoint_seq')
  int? get lastCheckpointSeq;

  /// Known value `checkpoint_sink_not_configured`.
  @BuiltValueField(wireName: r'warnings')
  BuiltList<String> get warnings;

  AuditVerifyResult._();

  factory AuditVerifyResult([void updates(AuditVerifyResultBuilder b)]) =
      _$AuditVerifyResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AuditVerifyResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AuditVerifyResult> get serializer =>
      _$AuditVerifyResultSerializer();
}

class _$AuditVerifyResultSerializer
    implements PrimitiveSerializer<AuditVerifyResult> {
  @override
  final Iterable<Type> types = const [AuditVerifyResult, _$AuditVerifyResult];

  @override
  final String wireName = r'AuditVerifyResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AuditVerifyResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'ok';
    yield serializers.serialize(object.ok, specifiedType: const FullType(bool));
    yield r'checked_from_seq';
    yield serializers.serialize(
      object.checkedFromSeq,
      specifiedType: const FullType(int),
    );
    yield r'checked_to_seq';
    yield serializers.serialize(
      object.checkedToSeq,
      specifiedType: const FullType(int),
    );
    if (object.firstBrokenSeq != null) {
      yield r'first_broken_seq';
      yield serializers.serialize(
        object.firstBrokenSeq,
        specifiedType: const FullType(int),
      );
    }
    if (object.failureReason != null) {
      yield r'failure_reason';
      yield serializers.serialize(
        object.failureReason,
        specifiedType: const FullType(AuditVerifyResultFailureReasonEnum),
      );
    }
    yield r'checkpoints_compared';
    yield serializers.serialize(
      object.checkpointsCompared,
      specifiedType: const FullType(int),
    );
    if (object.lastCheckpointSeq != null) {
      yield r'last_checkpoint_seq';
      yield serializers.serialize(
        object.lastCheckpointSeq,
        specifiedType: const FullType(int),
      );
    }
    yield r'warnings';
    yield serializers.serialize(
      object.warnings,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AuditVerifyResult object, {
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
    required AuditVerifyResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'ok':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ok = valueDes;
          break;
        case r'checked_from_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.checkedFromSeq = valueDes;
          break;
        case r'checked_to_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.checkedToSeq = valueDes;
          break;
        case r'first_broken_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.firstBrokenSeq = valueDes;
          break;
        case r'failure_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              AuditVerifyResultFailureReasonEnum,
            ),
          ) as AuditVerifyResultFailureReasonEnum?;
          if (valueDes == null) continue;
          result.failureReason = valueDes;
          break;
        case r'checkpoints_compared':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.checkpointsCompared = valueDes;
          break;
        case r'last_checkpoint_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.lastCheckpointSeq = valueDes;
          break;
        case r'warnings':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.warnings.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AuditVerifyResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AuditVerifyResultBuilder();
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

class AuditVerifyResultFailureReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'hash_mismatch')
  static const AuditVerifyResultFailureReasonEnum hashMismatch =
      _$auditVerifyResultFailureReasonEnum_hashMismatch;
  @BuiltValueEnumConst(wireName: r'link_mismatch')
  static const AuditVerifyResultFailureReasonEnum linkMismatch =
      _$auditVerifyResultFailureReasonEnum_linkMismatch;
  @BuiltValueEnumConst(wireName: r'seq_gap')
  static const AuditVerifyResultFailureReasonEnum seqGap =
      _$auditVerifyResultFailureReasonEnum_seqGap;
  @BuiltValueEnumConst(wireName: r'checkpoint_mismatch')
  static const AuditVerifyResultFailureReasonEnum checkpointMismatch =
      _$auditVerifyResultFailureReasonEnum_checkpointMismatch;
  @BuiltValueEnumConst(wireName: r'checkpoint_invalid')
  static const AuditVerifyResultFailureReasonEnum checkpointInvalid =
      _$auditVerifyResultFailureReasonEnum_checkpointInvalid;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AuditVerifyResultFailureReasonEnum unknownDefaultOpenApi =
      _$auditVerifyResultFailureReasonEnum_unknownDefaultOpenApi;

  static Serializer<AuditVerifyResultFailureReasonEnum> get serializer =>
      _$auditVerifyResultFailureReasonEnumSerializer;

  const AuditVerifyResultFailureReasonEnum._(String name) : super(name);

  static BuiltSet<AuditVerifyResultFailureReasonEnum> get values =>
      _$auditVerifyResultFailureReasonEnumValues;
  static AuditVerifyResultFailureReasonEnum valueOf(String name) =>
      _$auditVerifyResultFailureReasonEnumValueOf(name);
}
