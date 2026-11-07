//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_patch_result.g.dart';

/// RunPatchResult
///
/// Properties:
/// * [version]
/// * [autonomyCapReached] - Set when the patch's `/metrics/autonomy_last` made the idle count of the run's thread and person reach `run.autonomous_idle_max` (09 5.8.6); the runner closes stdin at that turn's `result`.
@BuiltValue()
abstract class RunPatchResult
    implements Built<RunPatchResult, RunPatchResultBuilder> {
  @BuiltValueField(wireName: r'version')
  int get version;

  /// Set when the patch's `/metrics/autonomy_last` made the idle count of the run's thread and person reach `run.autonomous_idle_max` (09 5.8.6); the runner closes stdin at that turn's `result`.
  @BuiltValueField(wireName: r'autonomy_cap_reached')
  bool? get autonomyCapReached;

  RunPatchResult._();

  factory RunPatchResult([void updates(RunPatchResultBuilder b)]) =
      _$RunPatchResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunPatchResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunPatchResult> get serializer =>
      _$RunPatchResultSerializer();
}

class _$RunPatchResultSerializer
    implements PrimitiveSerializer<RunPatchResult> {
  @override
  final Iterable<Type> types = const [RunPatchResult, _$RunPatchResult];

  @override
  final String wireName = r'RunPatchResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunPatchResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.autonomyCapReached != null) {
      yield r'autonomy_cap_reached';
      yield serializers.serialize(
        object.autonomyCapReached,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunPatchResult object, {
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
    required RunPatchResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'autonomy_cap_reached':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.autonomyCapReached = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunPatchResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunPatchResultBuilder();
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
