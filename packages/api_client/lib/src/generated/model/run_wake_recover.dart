//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_recover.g.dart';

/// RunWakeRecover
///
/// Properties:
/// * [reason]
/// * [label] - At most `run.wake_label_max_chars` characters, no control or bidi characters.
/// * [notBefore] - The earliest fire time, for `usage_limit` the reset plus `run.wake_min_delay`; never earlier than `run.wake_min_delay` after now.
@BuiltValue()
abstract class RunWakeRecover
    implements Built<RunWakeRecover, RunWakeRecoverBuilder> {
  @BuiltValueField(wireName: r'reason')
  RunWakeRecoverReasonEnum get reason;
  // enum reasonEnum {  recovery,  usage_limit,  };

  /// At most `run.wake_label_max_chars` characters, no control or bidi characters.
  @BuiltValueField(wireName: r'label')
  String? get label;

  /// The earliest fire time, for `usage_limit` the reset plus `run.wake_min_delay`; never earlier than `run.wake_min_delay` after now.
  @BuiltValueField(wireName: r'not_before')
  DateTime? get notBefore;

  RunWakeRecover._();

  factory RunWakeRecover([void updates(RunWakeRecoverBuilder b)]) =
      _$RunWakeRecover;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeRecoverBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeRecover> get serializer =>
      _$RunWakeRecoverSerializer();
}

class _$RunWakeRecoverSerializer
    implements PrimitiveSerializer<RunWakeRecover> {
  @override
  final Iterable<Type> types = const [RunWakeRecover, _$RunWakeRecover];

  @override
  final String wireName = r'RunWakeRecover';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeRecover object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(RunWakeRecoverReasonEnum),
    );
    if (object.label != null) {
      yield r'label';
      yield serializers.serialize(
        object.label,
        specifiedType: const FullType(String),
      );
    }
    if (object.notBefore != null) {
      yield r'not_before';
      yield serializers.serialize(
        object.notBefore,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeRecover object, {
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
    required RunWakeRecoverBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeRecoverReasonEnum),
          ) as RunWakeRecoverReasonEnum;
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
        case r'not_before':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.notBefore = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeRecover deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeRecoverBuilder();
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

class RunWakeRecoverReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'recovery')
  static const RunWakeRecoverReasonEnum recovery =
      _$runWakeRecoverReasonEnum_recovery;
  @BuiltValueEnumConst(wireName: r'usage_limit')
  static const RunWakeRecoverReasonEnum usageLimit =
      _$runWakeRecoverReasonEnum_usageLimit;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeRecoverReasonEnum unknownDefaultOpenApi =
      _$runWakeRecoverReasonEnum_unknownDefaultOpenApi;

  static Serializer<RunWakeRecoverReasonEnum> get serializer =>
      _$runWakeRecoverReasonEnumSerializer;

  const RunWakeRecoverReasonEnum._(String name) : super(name);

  static BuiltSet<RunWakeRecoverReasonEnum> get values =>
      _$runWakeRecoverReasonEnumValues;
  static RunWakeRecoverReasonEnum valueOf(String name) =>
      _$runWakeRecoverReasonEnumValueOf(name);
}
