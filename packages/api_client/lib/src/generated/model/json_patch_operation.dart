//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'json_patch_operation.g.dart';

/// One RFC 6902 operation, restricted to `add`, `replace` and `remove`.
///
/// Properties:
/// * [op]
/// * [path] - JSON Pointer (RFC 6901) into the run document.
/// * [value] - Any JSON value except null. The schema of the value is defined by its key.
@BuiltValue()
abstract class JsonPatchOperation
    implements Built<JsonPatchOperation, JsonPatchOperationBuilder> {
  @BuiltValueField(wireName: r'op')
  JsonPatchOperationOpEnum get op;
  // enum opEnum {  add,  replace,  remove,  };

  /// JSON Pointer (RFC 6901) into the run document.
  @BuiltValueField(wireName: r'path')
  String get path;

  /// Any JSON value except null. The schema of the value is defined by its key.
  @BuiltValueField(wireName: r'value')
  JsonObject? get value;

  JsonPatchOperation._();

  factory JsonPatchOperation([void updates(JsonPatchOperationBuilder b)]) =
      _$JsonPatchOperation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(JsonPatchOperationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<JsonPatchOperation> get serializer =>
      _$JsonPatchOperationSerializer();
}

class _$JsonPatchOperationSerializer
    implements PrimitiveSerializer<JsonPatchOperation> {
  @override
  final Iterable<Type> types = const [JsonPatchOperation, _$JsonPatchOperation];

  @override
  final String wireName = r'JsonPatchOperation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    JsonPatchOperation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'op';
    yield serializers.serialize(
      object.op,
      specifiedType: const FullType(JsonPatchOperationOpEnum),
    );
    yield r'path';
    yield serializers.serialize(
      object.path,
      specifiedType: const FullType(String),
    );
    if (object.value != null) {
      yield r'value';
      yield serializers.serialize(
        object.value,
        specifiedType: const FullType.nullable(JsonObject),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    JsonPatchOperation object, {
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
    required JsonPatchOperationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'op':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(JsonPatchOperationOpEnum),
          ) as JsonPatchOperationOpEnum;
          result.op = valueDes;
          break;
        case r'path':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.path = valueDes;
          break;
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(JsonObject),
          ) as JsonObject?;
          if (valueDes == null) continue;
          result.value = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  JsonPatchOperation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = JsonPatchOperationBuilder();
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

class JsonPatchOperationOpEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'add')
  static const JsonPatchOperationOpEnum add = _$jsonPatchOperationOpEnum_add;
  @BuiltValueEnumConst(wireName: r'replace')
  static const JsonPatchOperationOpEnum replace =
      _$jsonPatchOperationOpEnum_replace;
  @BuiltValueEnumConst(wireName: r'remove')
  static const JsonPatchOperationOpEnum remove =
      _$jsonPatchOperationOpEnum_remove;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const JsonPatchOperationOpEnum unknownDefaultOpenApi =
      _$jsonPatchOperationOpEnum_unknownDefaultOpenApi;

  static Serializer<JsonPatchOperationOpEnum> get serializer =>
      _$jsonPatchOperationOpEnumSerializer;

  const JsonPatchOperationOpEnum._(String name) : super(name);

  static BuiltSet<JsonPatchOperationOpEnum> get values =>
      _$jsonPatchOperationOpEnumValues;
  static JsonPatchOperationOpEnum valueOf(String name) =>
      _$jsonPatchOperationOpEnumValueOf(name);
}
