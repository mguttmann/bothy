//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/upload_session.dart';
import 'package:api_client/src/generated/model/import_item.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_file_slot.g.dart';

/// The `pending` item reserves the file ID; a repeated call with the same key returns the same ID (22, 7.2).
///
/// Properties:
/// * [item]
/// * [upload] - Absent when the file is already `posted`.
@BuiltValue()
abstract class ImportFileSlot
    implements Built<ImportFileSlot, ImportFileSlotBuilder> {
  @BuiltValueField(wireName: r'item')
  ImportItem get item;

  /// Absent when the file is already `posted`.
  @BuiltValueField(wireName: r'upload')
  UploadSession? get upload;

  ImportFileSlot._();

  factory ImportFileSlot([void updates(ImportFileSlotBuilder b)]) =
      _$ImportFileSlot;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportFileSlotBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportFileSlot> get serializer =>
      _$ImportFileSlotSerializer();
}

class _$ImportFileSlotSerializer
    implements PrimitiveSerializer<ImportFileSlot> {
  @override
  final Iterable<Type> types = const [ImportFileSlot, _$ImportFileSlot];

  @override
  final String wireName = r'ImportFileSlot';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportFileSlot object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'item';
    yield serializers.serialize(
      object.item,
      specifiedType: const FullType(ImportItem),
    );
    if (object.upload != null) {
      yield r'upload';
      yield serializers.serialize(
        object.upload,
        specifiedType: const FullType(UploadSession),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportFileSlot object, {
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
    required ImportFileSlotBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'item':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportItem),
          ) as ImportItem;
          result.item.replace(valueDes);
          break;
        case r'upload':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(UploadSession),
          ) as UploadSession?;
          if (valueDes == null) continue;
          result.upload.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportFileSlot deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportFileSlotBuilder();
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
