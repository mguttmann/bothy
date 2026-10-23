//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_pin_data.g.dart';

/// Data of a `pin` item (22, 6.6); `pinned_by` is the importing owner.
///
/// Properties:
/// * [messageOriginKey]
@BuiltValue()
abstract class ImportPinData
    implements Built<ImportPinData, ImportPinDataBuilder> {
  @BuiltValueField(wireName: r'message_origin_key')
  String get messageOriginKey;

  ImportPinData._();

  factory ImportPinData([void updates(ImportPinDataBuilder b)]) =
      _$ImportPinData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportPinDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportPinData> get serializer =>
      _$ImportPinDataSerializer();
}

class _$ImportPinDataSerializer implements PrimitiveSerializer<ImportPinData> {
  @override
  final Iterable<Type> types = const [ImportPinData, _$ImportPinData];

  @override
  final String wireName = r'ImportPinData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportPinData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_origin_key';
    yield serializers.serialize(
      object.messageOriginKey,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportPinData object, {
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
    required ImportPinDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageOriginKey = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportPinData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportPinDataBuilder();
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
