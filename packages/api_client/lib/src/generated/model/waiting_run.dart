//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'waiting_run.g.dart';

/// One run an adapter accepted that waits before its start (09 5.2.1).
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason] - `slots_full`: every slot of `runner.max_parallel_runs` is in use; `thread_busy`: the thread or session lock is held by another turn.
@BuiltValue()
abstract class WaitingRun implements Built<WaitingRun, WaitingRunBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// `slots_full`: every slot of `runner.max_parallel_runs` is in use; `thread_busy`: the thread or session lock is held by another turn.
  @BuiltValueField(wireName: r'reason')
  WaitingRunReasonEnum get reason;
  // enum reasonEnum {  slots_full,  thread_busy,  };

  WaitingRun._();

  factory WaitingRun([void updates(WaitingRunBuilder b)]) = _$WaitingRun;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WaitingRunBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WaitingRun> get serializer => _$WaitingRunSerializer();
}

class _$WaitingRunSerializer implements PrimitiveSerializer<WaitingRun> {
  @override
  final Iterable<Type> types = const [WaitingRun, _$WaitingRun];

  @override
  final String wireName = r'WaitingRun';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WaitingRun object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(WaitingRunReasonEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    WaitingRun object, {
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
    required WaitingRunBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(WaitingRunReasonEnum),
          ) as WaitingRunReasonEnum;
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
  WaitingRun deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WaitingRunBuilder();
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

/// `slots_full`: every slot of `runner.max_parallel_runs` is in use; `thread_busy`: the thread or session lock is held by another turn.
class WaitingRunReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'slots_full')
  static const WaitingRunReasonEnum slotsFull =
      _$waitingRunReasonEnum_slotsFull;
  @BuiltValueEnumConst(wireName: r'thread_busy')
  static const WaitingRunReasonEnum threadBusy =
      _$waitingRunReasonEnum_threadBusy;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WaitingRunReasonEnum unknownDefaultOpenApi =
      _$waitingRunReasonEnum_unknownDefaultOpenApi;

  static Serializer<WaitingRunReasonEnum> get serializer =>
      _$waitingRunReasonEnumSerializer;

  const WaitingRunReasonEnum._(String name) : super(name);

  static BuiltSet<WaitingRunReasonEnum> get values =>
      _$waitingRunReasonEnumValues;
  static WaitingRunReasonEnum valueOf(String name) =>
      _$waitingRunReasonEnumValueOf(name);
}
