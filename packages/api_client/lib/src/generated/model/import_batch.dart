//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/import_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_batch.g.dart';

/// One transaction. Objects of one thread, in source order; structure objects (space, channel, forum tag, person) come in batches without `thread_origin_key`.
///
/// Properties:
/// * [threadOriginKey]
/// * [items] - At most `import.batch_max_items`.
@BuiltValue()
abstract class ImportBatch implements Built<ImportBatch, ImportBatchBuilder> {
  @BuiltValueField(wireName: r'thread_origin_key')
  String? get threadOriginKey;

  /// At most `import.batch_max_items`.
  @BuiltValueField(wireName: r'items')
  BuiltList<ImportObject> get items;

  ImportBatch._();

  factory ImportBatch([void updates(ImportBatchBuilder b)]) = _$ImportBatch;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportBatchBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportBatch> get serializer => _$ImportBatchSerializer();
}

class _$ImportBatchSerializer implements PrimitiveSerializer<ImportBatch> {
  @override
  final Iterable<Type> types = const [ImportBatch, _$ImportBatch];

  @override
  final String wireName = r'ImportBatch';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportBatch object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.threadOriginKey != null) {
      yield r'thread_origin_key';
      yield serializers.serialize(
        object.threadOriginKey,
        specifiedType: const FullType(String),
      );
    }
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ImportObject)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportBatch object, {
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
    required ImportBatchBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadOriginKey = valueDes;
          break;
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ImportObject)]),
          ) as BuiltList<ImportObject>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportBatch deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportBatchBuilder();
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
