//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/import_item.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_batch_result.g.dart';

/// ImportBatchResult
///
/// Properties:
/// * [items]
/// * [unreadFloorSeq] - New `threads.unread_floor_seq` of the thread of this batch.
@BuiltValue()
abstract class ImportBatchResult
    implements Built<ImportBatchResult, ImportBatchResultBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ImportItem> get items;

  /// New `threads.unread_floor_seq` of the thread of this batch.
  @BuiltValueField(wireName: r'unread_floor_seq')
  int? get unreadFloorSeq;

  ImportBatchResult._();

  factory ImportBatchResult([void updates(ImportBatchResultBuilder b)]) =
      _$ImportBatchResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportBatchResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportBatchResult> get serializer =>
      _$ImportBatchResultSerializer();
}

class _$ImportBatchResultSerializer
    implements PrimitiveSerializer<ImportBatchResult> {
  @override
  final Iterable<Type> types = const [ImportBatchResult, _$ImportBatchResult];

  @override
  final String wireName = r'ImportBatchResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportBatchResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ImportItem)]),
    );
    if (object.unreadFloorSeq != null) {
      yield r'unread_floor_seq';
      yield serializers.serialize(
        object.unreadFloorSeq,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportBatchResult object, {
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
    required ImportBatchResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ImportItem)]),
          ) as BuiltList<ImportItem>;
          result.items.replace(valueDes);
          break;
        case r'unread_floor_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.unreadFloorSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportBatchResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportBatchResultBuilder();
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
