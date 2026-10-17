//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/json_patch_operation.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_run_update.g.dart';

/// GatewayRunUpdate
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [baseVersion]
/// * [ops]
/// * [ref] - Client reference echoed in `error` frames for this op.
@BuiltValue()
abstract class GatewayRunUpdate
    implements Built<GatewayRunUpdate, GatewayRunUpdateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'base_version')
  int get baseVersion;

  @BuiltValueField(wireName: r'ops')
  BuiltList<JsonPatchOperation> get ops;

  /// Client reference echoed in `error` frames for this op.
  @BuiltValueField(wireName: r'ref')
  String? get ref;

  GatewayRunUpdate._();

  factory GatewayRunUpdate([void updates(GatewayRunUpdateBuilder b)]) =
      _$GatewayRunUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayRunUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayRunUpdate> get serializer =>
      _$GatewayRunUpdateSerializer();
}

class _$GatewayRunUpdateSerializer
    implements PrimitiveSerializer<GatewayRunUpdate> {
  @override
  final Iterable<Type> types = const [GatewayRunUpdate, _$GatewayRunUpdate];

  @override
  final String wireName = r'GatewayRunUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayRunUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
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
    if (object.ref != null) {
      yield r'ref';
      yield serializers.serialize(
        object.ref,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayRunUpdate object, {
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
    required GatewayRunUpdateBuilder result,
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
        case r'ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.ref = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayRunUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayRunUpdateBuilder();
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
