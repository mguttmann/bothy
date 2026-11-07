//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_autonomy_last.g.dart';

/// The idle flag of the last autonomous input or notification turn (09 5.8.6); the server counts each distinct `input` once in the idle count of the run's thread and person.
///
/// Properties:
/// * [input] - The wake-up id of a wake run or wake input, or the runner's id of a notification turn.
/// * [idle] - True when the input made no tool call other than `bothy-wake` and started no background task.
@BuiltValue()
abstract class RunAutonomyLast
    implements Built<RunAutonomyLast, RunAutonomyLastBuilder> {
  /// The wake-up id of a wake run or wake input, or the runner's id of a notification turn.
  @BuiltValueField(wireName: r'input')
  String get input;

  /// True when the input made no tool call other than `bothy-wake` and started no background task.
  @BuiltValueField(wireName: r'idle')
  bool get idle;

  RunAutonomyLast._();

  factory RunAutonomyLast([void updates(RunAutonomyLastBuilder b)]) =
      _$RunAutonomyLast;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunAutonomyLastBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunAutonomyLast> get serializer =>
      _$RunAutonomyLastSerializer();
}

class _$RunAutonomyLastSerializer
    implements PrimitiveSerializer<RunAutonomyLast> {
  @override
  final Iterable<Type> types = const [RunAutonomyLast, _$RunAutonomyLast];

  @override
  final String wireName = r'RunAutonomyLast';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunAutonomyLast object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'input';
    yield serializers.serialize(
      object.input,
      specifiedType: const FullType(String),
    );
    yield r'idle';
    yield serializers.serialize(
      object.idle,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunAutonomyLast object, {
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
    required RunAutonomyLastBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'input':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.input = valueDes;
          break;
        case r'idle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.idle = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunAutonomyLast deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunAutonomyLastBuilder();
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
