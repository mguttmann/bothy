//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/json_patch_operation.dart';
import 'package:api_client/src/generated/model/run.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_updated_payload.g.dart';

/// Coalesced live-box patch. Either `ops` (from `base_version` to `version`) or a full `snapshot`.
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [baseVersion]
/// * [version]
/// * [ops]
/// * [snapshot]
@BuiltValue()
abstract class RunUpdatedPayload
    implements Built<RunUpdatedPayload, RunUpdatedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'base_version')
  int? get baseVersion;

  @BuiltValueField(wireName: r'version')
  int get version;

  @BuiltValueField(wireName: r'ops')
  BuiltList<JsonPatchOperation>? get ops;

  @BuiltValueField(wireName: r'snapshot')
  Run? get snapshot;

  RunUpdatedPayload._();

  factory RunUpdatedPayload([void updates(RunUpdatedPayloadBuilder b)]) =
      _$RunUpdatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunUpdatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunUpdatedPayload> get serializer =>
      _$RunUpdatedPayloadSerializer();
}

class _$RunUpdatedPayloadSerializer
    implements PrimitiveSerializer<RunUpdatedPayload> {
  @override
  final Iterable<Type> types = const [RunUpdatedPayload, _$RunUpdatedPayload];

  @override
  final String wireName = r'RunUpdatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunUpdatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    if (object.baseVersion != null) {
      yield r'base_version';
      yield serializers.serialize(
        object.baseVersion,
        specifiedType: const FullType(int),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.ops != null) {
      yield r'ops';
      yield serializers.serialize(
        object.ops,
        specifiedType: const FullType(BuiltList, [
          FullType(JsonPatchOperation),
        ]),
      );
    }
    if (object.snapshot != null) {
      yield r'snapshot';
      yield serializers.serialize(
        object.snapshot,
        specifiedType: const FullType(Run),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunUpdatedPayload object, {
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
    required RunUpdatedPayloadBuilder result,
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
        case r'base_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.baseVersion = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'ops':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(JsonPatchOperation),
            ]),
          ) as BuiltList<JsonPatchOperation>?;
          if (valueDes == null) continue;
          result.ops.replace(valueDes);
          break;
        case r'snapshot':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Run),
          ) as Run?;
          if (valueDes == null) continue;
          result.snapshot.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunUpdatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunUpdatedPayloadBuilder();
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
