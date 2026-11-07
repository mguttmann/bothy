//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_condition.g.dart';

/// Schema `run_wake_condition.v1` (03 `run_wakes`); `repository`, `number` and `head_commit` for `pr_checks`, `command_line` for `command`, nothing for `at`.
///
/// Properties:
/// * [repository]
/// * [number]
/// * [headCommit] - The head commit of the pull request at the creation.
/// * [commandLine] - Run with `/bin/sh -c` in the turn's working folder; fires on exit code 0. At most `message.max_content_bytes` bytes, no control or bidi characters except line feed and tab.
@BuiltValue()
abstract class RunWakeCondition
    implements Built<RunWakeCondition, RunWakeConditionBuilder> {
  @BuiltValueField(wireName: r'repository')
  String? get repository;

  @BuiltValueField(wireName: r'number')
  int? get number;

  /// The head commit of the pull request at the creation.
  @BuiltValueField(wireName: r'head_commit')
  String? get headCommit;

  /// Run with `/bin/sh -c` in the turn's working folder; fires on exit code 0. At most `message.max_content_bytes` bytes, no control or bidi characters except line feed and tab.
  @BuiltValueField(wireName: r'command_line')
  String? get commandLine;

  RunWakeCondition._();

  factory RunWakeCondition([void updates(RunWakeConditionBuilder b)]) =
      _$RunWakeCondition;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeConditionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeCondition> get serializer =>
      _$RunWakeConditionSerializer();
}

class _$RunWakeConditionSerializer
    implements PrimitiveSerializer<RunWakeCondition> {
  @override
  final Iterable<Type> types = const [RunWakeCondition, _$RunWakeCondition];

  @override
  final String wireName = r'RunWakeCondition';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeCondition object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.repository != null) {
      yield r'repository';
      yield serializers.serialize(
        object.repository,
        specifiedType: const FullType(String),
      );
    }
    if (object.number != null) {
      yield r'number';
      yield serializers.serialize(
        object.number,
        specifiedType: const FullType(int),
      );
    }
    if (object.headCommit != null) {
      yield r'head_commit';
      yield serializers.serialize(
        object.headCommit,
        specifiedType: const FullType(String),
      );
    }
    if (object.commandLine != null) {
      yield r'command_line';
      yield serializers.serialize(
        object.commandLine,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeCondition object, {
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
    required RunWakeConditionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'repository':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.repository = valueDes;
          break;
        case r'number':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.number = valueDes;
          break;
        case r'head_commit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.headCommit = valueDes;
          break;
        case r'command_line':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.commandLine = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeCondition deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeConditionBuilder();
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
