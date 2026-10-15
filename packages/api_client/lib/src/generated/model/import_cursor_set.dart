//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_cursor_set.g.dart';

/// ImportCursorSet
///
/// Properties:
/// * [cursor] - Progress per source channel and thread; only after a successful batch.
/// * [stats] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class ImportCursorSet
    implements Built<ImportCursorSet, ImportCursorSetBuilder> {
  /// Progress per source channel and thread; only after a successful batch.
  @BuiltValueField(wireName: r'cursor')
  BuiltMap<String, JsonObject?> get cursor;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'stats')
  BuiltMap<String, JsonObject?>? get stats;

  ImportCursorSet._();

  factory ImportCursorSet([void updates(ImportCursorSetBuilder b)]) =
      _$ImportCursorSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportCursorSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportCursorSet> get serializer =>
      _$ImportCursorSetSerializer();
}

class _$ImportCursorSetSerializer
    implements PrimitiveSerializer<ImportCursorSet> {
  @override
  final Iterable<Type> types = const [ImportCursorSet, _$ImportCursorSet];

  @override
  final String wireName = r'ImportCursorSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportCursorSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'cursor';
    yield serializers.serialize(
      object.cursor,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    if (object.stats != null) {
      yield r'stats';
      yield serializers.serialize(
        object.stats,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportCursorSet object, {
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
    required ImportCursorSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.cursor.replace(valueDes);
          break;
        case r'stats':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.stats.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportCursorSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportCursorSetBuilder();
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
