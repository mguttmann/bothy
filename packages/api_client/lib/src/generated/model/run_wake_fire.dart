//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_fire.g.dart';

/// RunWakeFire
///
/// Properties:
/// * [outcome]
/// * [resultText] - The result of `fired`; at most `message.max_content_bytes` bytes, no control or bidi characters except line feed and tab.
/// * [reason] - The reason of `failed` (the last failure, for example `gh_missing`); absent means `wake_check_failed`.
/// * [runId] - The run of a skipped wake input; required for `input_skipped`.
@BuiltValue()
abstract class RunWakeFire implements Built<RunWakeFire, RunWakeFireBuilder> {
  @BuiltValueField(wireName: r'outcome')
  RunWakeFireOutcomeEnum get outcome;
  // enum outcomeEnum {  fired,  failed,  input_skipped,  };

  /// The result of `fired`; at most `message.max_content_bytes` bytes, no control or bidi characters except line feed and tab.
  @BuiltValueField(wireName: r'result_text')
  String? get resultText;

  /// The reason of `failed` (the last failure, for example `gh_missing`); absent means `wake_check_failed`.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  /// The run of a skipped wake input; required for `input_skipped`.
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  RunWakeFire._();

  factory RunWakeFire([void updates(RunWakeFireBuilder b)]) = _$RunWakeFire;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeFireBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeFire> get serializer => _$RunWakeFireSerializer();
}

class _$RunWakeFireSerializer implements PrimitiveSerializer<RunWakeFire> {
  @override
  final Iterable<Type> types = const [RunWakeFire, _$RunWakeFire];

  @override
  final String wireName = r'RunWakeFire';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeFire object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'outcome';
    yield serializers.serialize(
      object.outcome,
      specifiedType: const FullType(RunWakeFireOutcomeEnum),
    );
    if (object.resultText != null) {
      yield r'result_text';
      yield serializers.serialize(
        object.resultText,
        specifiedType: const FullType(String),
      );
    }
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeFire object, {
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
    required RunWakeFireBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'outcome':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeFireOutcomeEnum),
          ) as RunWakeFireOutcomeEnum;
          result.outcome = valueDes;
          break;
        case r'result_text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.resultText = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeFire deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeFireBuilder();
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

class RunWakeFireOutcomeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'fired')
  static const RunWakeFireOutcomeEnum fired = _$runWakeFireOutcomeEnum_fired;
  @BuiltValueEnumConst(wireName: r'failed')
  static const RunWakeFireOutcomeEnum failed = _$runWakeFireOutcomeEnum_failed;
  @BuiltValueEnumConst(wireName: r'input_skipped')
  static const RunWakeFireOutcomeEnum inputSkipped =
      _$runWakeFireOutcomeEnum_inputSkipped;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeFireOutcomeEnum unknownDefaultOpenApi =
      _$runWakeFireOutcomeEnum_unknownDefaultOpenApi;

  static Serializer<RunWakeFireOutcomeEnum> get serializer =>
      _$runWakeFireOutcomeEnumSerializer;

  const RunWakeFireOutcomeEnum._(String name) : super(name);

  static BuiltSet<RunWakeFireOutcomeEnum> get values =>
      _$runWakeFireOutcomeEnumValues;
  static RunWakeFireOutcomeEnum valueOf(String name) =>
      _$runWakeFireOutcomeEnumValueOf(name);
}
