//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_reconcile_result.g.dart';

/// CostReconcileResult
///
/// Properties:
/// * [reconciled]
/// * [unmatchedRunIds]
@BuiltValue()
abstract class CostReconcileResult
    implements Built<CostReconcileResult, CostReconcileResultBuilder> {
  @BuiltValueField(wireName: r'reconciled')
  int get reconciled;

  @BuiltValueField(wireName: r'unmatched_run_ids')
  BuiltList<String> get unmatchedRunIds;

  CostReconcileResult._();

  factory CostReconcileResult([void updates(CostReconcileResultBuilder b)]) =
      _$CostReconcileResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostReconcileResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostReconcileResult> get serializer =>
      _$CostReconcileResultSerializer();
}

class _$CostReconcileResultSerializer
    implements PrimitiveSerializer<CostReconcileResult> {
  @override
  final Iterable<Type> types = const [
    CostReconcileResult,
    _$CostReconcileResult,
  ];

  @override
  final String wireName = r'CostReconcileResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostReconcileResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'reconciled';
    yield serializers.serialize(
      object.reconciled,
      specifiedType: const FullType(int),
    );
    yield r'unmatched_run_ids';
    yield serializers.serialize(
      object.unmatchedRunIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostReconcileResult object, {
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
    required CostReconcileResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'reconciled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.reconciled = valueDes;
          break;
        case r'unmatched_run_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.unmatchedRunIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostReconcileResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostReconcileResultBuilder();
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
