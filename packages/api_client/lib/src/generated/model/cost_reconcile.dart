//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/reconcile_entry.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_reconcile.g.dart';

/// CostReconcile
///
/// Properties:
/// * [provider]
/// * [periodStart] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [periodEnd] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [entries]
@BuiltValue()
abstract class CostReconcile
    implements Built<CostReconcile, CostReconcileBuilder> {
  @BuiltValueField(wireName: r'provider')
  String get provider;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'period_start')
  DateTime get periodStart;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'period_end')
  DateTime get periodEnd;

  @BuiltValueField(wireName: r'entries')
  BuiltList<ReconcileEntry> get entries;

  CostReconcile._();

  factory CostReconcile([void updates(CostReconcileBuilder b)]) =
      _$CostReconcile;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostReconcileBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostReconcile> get serializer =>
      _$CostReconcileSerializer();
}

class _$CostReconcileSerializer implements PrimitiveSerializer<CostReconcile> {
  @override
  final Iterable<Type> types = const [CostReconcile, _$CostReconcile];

  @override
  final String wireName = r'CostReconcile';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostReconcile object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(String),
    );
    yield r'period_start';
    yield serializers.serialize(
      object.periodStart,
      specifiedType: const FullType(DateTime),
    );
    yield r'period_end';
    yield serializers.serialize(
      object.periodEnd,
      specifiedType: const FullType(DateTime),
    );
    yield r'entries';
    yield serializers.serialize(
      object.entries,
      specifiedType: const FullType(BuiltList, [FullType(ReconcileEntry)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostReconcile object, {
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
    required CostReconcileBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.provider = valueDes;
          break;
        case r'period_start':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.periodStart = valueDes;
          break;
        case r'period_end':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.periodEnd = valueDes;
          break;
        case r'entries':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(ReconcileEntry),
            ]),
          ) as BuiltList<ReconcileEntry>;
          result.entries.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostReconcile deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostReconcileBuilder();
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
