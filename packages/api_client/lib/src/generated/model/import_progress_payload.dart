//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/import_source.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_progress_payload.g.dart';

/// ImportProgressPayload
///
/// Properties:
/// * [import_]
@BuiltValue()
abstract class ImportProgressPayload
    implements Built<ImportProgressPayload, ImportProgressPayloadBuilder> {
  @BuiltValueField(wireName: r'import')
  ImportSource get import_;

  ImportProgressPayload._();

  factory ImportProgressPayload([
    void updates(ImportProgressPayloadBuilder b),
  ]) = _$ImportProgressPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportProgressPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportProgressPayload> get serializer =>
      _$ImportProgressPayloadSerializer();
}

class _$ImportProgressPayloadSerializer
    implements PrimitiveSerializer<ImportProgressPayload> {
  @override
  final Iterable<Type> types = const [
    ImportProgressPayload,
    _$ImportProgressPayload,
  ];

  @override
  final String wireName = r'ImportProgressPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportProgressPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'import';
    yield serializers.serialize(
      object.import_,
      specifiedType: const FullType(ImportSource),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportProgressPayload object, {
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
    required ImportProgressPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'import':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportSource),
          ) as ImportSource;
          result.import_.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportProgressPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportProgressPayloadBuilder();
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
