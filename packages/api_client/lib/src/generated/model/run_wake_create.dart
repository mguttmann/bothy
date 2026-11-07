//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_wake_condition.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_wake_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_create.g.dart';

/// RunWakeCreate
///
/// Properties:
/// * [kind]
/// * [reason]
/// * [label] - At most `run.wake_label_max_chars` characters, no control or bidi characters.
/// * [condition]
/// * [notBefore] - The fire time of an `at` wake-up; never earlier than `run.wake_min_delay` after now. For `usage_limit` the reset time plus `run.wake_min_delay`.
/// * [deadline] - At most `run.wake_max_wait` after now, for `usage_limit` after `not_before`; absent means that longest deadline.
/// * [outage] - The run's outage number; required exactly for `usage_limit` (18.19.10).
/// * [deferred_] - Set by the SDK outbox for a request it sends later; a refusal is then stored in state `refused` with the note and the notification (18.19.8).
@BuiltValue()
abstract class RunWakeCreate
    implements Built<RunWakeCreate, RunWakeCreateBuilder> {
  @BuiltValueField(wireName: r'kind')
  RunWakeKind get kind;
  // enum kindEnum {  at,  pr_checks,  command,  };

  @BuiltValueField(wireName: r'reason')
  RunWakeCreateReasonEnum get reason;
  // enum reasonEnum {  agent,  usage_limit,  };

  /// At most `run.wake_label_max_chars` characters, no control or bidi characters.
  @BuiltValueField(wireName: r'label')
  String? get label;

  @BuiltValueField(wireName: r'condition')
  RunWakeCondition? get condition;

  /// The fire time of an `at` wake-up; never earlier than `run.wake_min_delay` after now. For `usage_limit` the reset time plus `run.wake_min_delay`.
  @BuiltValueField(wireName: r'not_before')
  DateTime? get notBefore;

  /// At most `run.wake_max_wait` after now, for `usage_limit` after `not_before`; absent means that longest deadline.
  @BuiltValueField(wireName: r'deadline')
  DateTime? get deadline;

  /// The run's outage number; required exactly for `usage_limit` (18.19.10).
  @BuiltValueField(wireName: r'outage')
  int? get outage;

  /// Set by the SDK outbox for a request it sends later; a refusal is then stored in state `refused` with the note and the notification (18.19.8).
  @BuiltValueField(wireName: r'deferred')
  bool? get deferred_;

  RunWakeCreate._();

  factory RunWakeCreate([void updates(RunWakeCreateBuilder b)]) =
      _$RunWakeCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeCreate> get serializer =>
      _$RunWakeCreateSerializer();
}

class _$RunWakeCreateSerializer implements PrimitiveSerializer<RunWakeCreate> {
  @override
  final Iterable<Type> types = const [RunWakeCreate, _$RunWakeCreate];

  @override
  final String wireName = r'RunWakeCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(RunWakeKind),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(RunWakeCreateReasonEnum),
    );
    if (object.label != null) {
      yield r'label';
      yield serializers.serialize(
        object.label,
        specifiedType: const FullType(String),
      );
    }
    if (object.condition != null) {
      yield r'condition';
      yield serializers.serialize(
        object.condition,
        specifiedType: const FullType(RunWakeCondition),
      );
    }
    if (object.notBefore != null) {
      yield r'not_before';
      yield serializers.serialize(
        object.notBefore,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.deadline != null) {
      yield r'deadline';
      yield serializers.serialize(
        object.deadline,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.outage != null) {
      yield r'outage';
      yield serializers.serialize(
        object.outage,
        specifiedType: const FullType(int),
      );
    }
    if (object.deferred_ != null) {
      yield r'deferred';
      yield serializers.serialize(
        object.deferred_,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeCreate object, {
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
    required RunWakeCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeKind),
          ) as RunWakeKind;
          result.kind = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeCreateReasonEnum),
          ) as RunWakeCreateReasonEnum;
          result.reason = valueDes;
          break;
        case r'label':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.label = valueDes;
          break;
        case r'condition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunWakeCondition),
          ) as RunWakeCondition?;
          if (valueDes == null) continue;
          result.condition.replace(valueDes);
          break;
        case r'not_before':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.notBefore = valueDes;
          break;
        case r'deadline':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.deadline = valueDes;
          break;
        case r'outage':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.outage = valueDes;
          break;
        case r'deferred':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.deferred_ = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeCreateBuilder();
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

class RunWakeCreateReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'agent')
  static const RunWakeCreateReasonEnum agent = _$runWakeCreateReasonEnum_agent;
  @BuiltValueEnumConst(wireName: r'usage_limit')
  static const RunWakeCreateReasonEnum usageLimit =
      _$runWakeCreateReasonEnum_usageLimit;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeCreateReasonEnum unknownDefaultOpenApi =
      _$runWakeCreateReasonEnum_unknownDefaultOpenApi;

  static Serializer<RunWakeCreateReasonEnum> get serializer =>
      _$runWakeCreateReasonEnumSerializer;

  const RunWakeCreateReasonEnum._(String name) : super(name);

  static BuiltSet<RunWakeCreateReasonEnum> get values =>
      _$runWakeCreateReasonEnumValues;
  static RunWakeCreateReasonEnum valueOf(String name) =>
      _$runWakeCreateReasonEnumValueOf(name);
}
