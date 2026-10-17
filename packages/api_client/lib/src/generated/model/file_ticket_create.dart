//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'file_ticket_create.g.dart';

/// FileTicketCreate
///
/// Properties:
/// * [variant]
@BuiltValue()
abstract class FileTicketCreate
    implements Built<FileTicketCreate, FileTicketCreateBuilder> {
  @BuiltValueField(wireName: r'variant')
  FileTicketCreateVariantEnum get variant;
  // enum variantEnum {  original,  small,  medium,  large,  poster,  };

  FileTicketCreate._();

  factory FileTicketCreate([void updates(FileTicketCreateBuilder b)]) =
      _$FileTicketCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FileTicketCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FileTicketCreate> get serializer =>
      _$FileTicketCreateSerializer();
}

class _$FileTicketCreateSerializer
    implements PrimitiveSerializer<FileTicketCreate> {
  @override
  final Iterable<Type> types = const [FileTicketCreate, _$FileTicketCreate];

  @override
  final String wireName = r'FileTicketCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FileTicketCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'variant';
    yield serializers.serialize(
      object.variant,
      specifiedType: const FullType(FileTicketCreateVariantEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    FileTicketCreate object, {
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
    required FileTicketCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'variant':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(FileTicketCreateVariantEnum),
          ) as FileTicketCreateVariantEnum;
          result.variant = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  FileTicketCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FileTicketCreateBuilder();
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

class FileTicketCreateVariantEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'original')
  static const FileTicketCreateVariantEnum original =
      _$fileTicketCreateVariantEnum_original;
  @BuiltValueEnumConst(wireName: r'small')
  static const FileTicketCreateVariantEnum small =
      _$fileTicketCreateVariantEnum_small;
  @BuiltValueEnumConst(wireName: r'medium')
  static const FileTicketCreateVariantEnum medium =
      _$fileTicketCreateVariantEnum_medium;
  @BuiltValueEnumConst(wireName: r'large')
  static const FileTicketCreateVariantEnum large =
      _$fileTicketCreateVariantEnum_large;
  @BuiltValueEnumConst(wireName: r'poster')
  static const FileTicketCreateVariantEnum poster =
      _$fileTicketCreateVariantEnum_poster;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const FileTicketCreateVariantEnum unknownDefaultOpenApi =
      _$fileTicketCreateVariantEnum_unknownDefaultOpenApi;

  static Serializer<FileTicketCreateVariantEnum> get serializer =>
      _$fileTicketCreateVariantEnumSerializer;

  const FileTicketCreateVariantEnum._(String name) : super(name);

  static BuiltSet<FileTicketCreateVariantEnum> get values =>
      _$fileTicketCreateVariantEnumValues;
  static FileTicketCreateVariantEnum valueOf(String name) =>
      _$fileTicketCreateVariantEnumValueOf(name);
}
