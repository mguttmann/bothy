//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/json_patch_operation.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_patch.g.dart';

/// RunPatch
///
/// Properties:
/// * [baseVersion]
/// * [ops]
@BuiltValue()
abstract class RunPatch implements Built<RunPatch, RunPatchBuilder> {
  @BuiltValueField(wireName: r'base_version')
  int get baseVersion;

  @BuiltValueField(wireName: r'ops')
  BuiltList<JsonPatchOperation> get ops;

  RunPatch._();

  factory RunPatch([void updates(RunPatchBuilder b)]) = _$RunPatch;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunPatchBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunPatch> get serializer => _$RunPatchSerializer();
}

class _$RunPatchSerializer implements PrimitiveSerializer<RunPatch> {
  @override
  final Iterable<Type> types = const [RunPatch, _$RunPatch];

  @override
  final String wireName = r'RunPatch';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunPatch object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'base_version';
    yield serializers.serialize(
      object.baseVersion,
      specifiedType: const FullType(int),
    );
    yield r'ops';
    yield serializers.serialize(
      object.ops,
      specifiedType: const FullType(BuiltList, [FullType(JsonPatchOperation)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunPatch object, {
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
    required RunPatchBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'base_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.baseVersion = valueDes;
          break;
        case r'ops':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(JsonPatchOperation),
            ]),
          ) as BuiltList<JsonPatchOperation>;
          result.ops.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunPatch deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunPatchBuilder();
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
