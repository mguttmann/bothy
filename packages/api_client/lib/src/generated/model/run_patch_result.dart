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
@BuiltValue()
abstract class RunPatchResult
    implements Built<RunPatchResult, RunPatchResultBuilder> {
  @BuiltValueField(wireName: r'version')
  int get version;

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
