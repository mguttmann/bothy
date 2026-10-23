//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_space_data.g.dart';

/// `data` of a `space` item (origin key `space:<category_id>`). Exactly one of `name` (a new space as the mapping says `create`) and `existing_space_id` (equal to the mapping's `existing`).
///
/// Properties:
/// * [name]
/// * [sortKey] - Fractional index key of the space order, derived by the tool from the Discord `position`.
/// * [existingSpaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ImportSpaceData
    implements Built<ImportSpaceData, ImportSpaceDataBuilder> {
  @BuiltValueField(wireName: r'name')
  String? get name;

  /// Fractional index key of the space order, derived by the tool from the Discord `position`.
  @BuiltValueField(wireName: r'sort_key')
  String get sortKey;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'existing_space_id')
  String? get existingSpaceId;

  ImportSpaceData._();

  factory ImportSpaceData([void updates(ImportSpaceDataBuilder b)]) =
      _$ImportSpaceData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSpaceDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSpaceData> get serializer =>
      _$ImportSpaceDataSerializer();
}

class _$ImportSpaceDataSerializer
    implements PrimitiveSerializer<ImportSpaceData> {
  @override
  final Iterable<Type> types = const [ImportSpaceData, _$ImportSpaceData];

  @override
  final String wireName = r'ImportSpaceData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSpaceData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    yield r'sort_key';
    yield serializers.serialize(
      object.sortKey,
      specifiedType: const FullType(String),
    );
    if (object.existingSpaceId != null) {
      yield r'existing_space_id';
      yield serializers.serialize(
        object.existingSpaceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSpaceData object, {
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
    required ImportSpaceDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.name = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sortKey = valueDes;
          break;
        case r'existing_space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.existingSpaceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSpaceData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSpaceDataBuilder();
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
