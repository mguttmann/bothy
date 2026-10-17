//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_lead_set.g.dart';

/// ImportLeadSet
///
/// Properties:
/// * [sourceChannel] - ID of the channel in the source.
/// * [lead] - `discord`: the source leads, the channel here is read-only and mirrored; `bothy`: this server leads (22, 12.1).
@BuiltValue()
abstract class ImportLeadSet
    implements Built<ImportLeadSet, ImportLeadSetBuilder> {
  /// ID of the channel in the source.
  @BuiltValueField(wireName: r'source_channel')
  String get sourceChannel;

  /// `discord`: the source leads, the channel here is read-only and mirrored; `bothy`: this server leads (22, 12.1).
  @BuiltValueField(wireName: r'lead')
  ImportLeadSetLeadEnum get lead;
  // enum leadEnum {  discord,  bothy,  };

  ImportLeadSet._();

  factory ImportLeadSet([void updates(ImportLeadSetBuilder b)]) =
      _$ImportLeadSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportLeadSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportLeadSet> get serializer =>
      _$ImportLeadSetSerializer();
}

class _$ImportLeadSetSerializer implements PrimitiveSerializer<ImportLeadSet> {
  @override
  final Iterable<Type> types = const [ImportLeadSet, _$ImportLeadSet];

  @override
  final String wireName = r'ImportLeadSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportLeadSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'source_channel';
    yield serializers.serialize(
      object.sourceChannel,
      specifiedType: const FullType(String),
    );
    yield r'lead';
    yield serializers.serialize(
      object.lead,
      specifiedType: const FullType(ImportLeadSetLeadEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportLeadSet object, {
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
    required ImportLeadSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'source_channel':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sourceChannel = valueDes;
          break;
        case r'lead':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportLeadSetLeadEnum),
          ) as ImportLeadSetLeadEnum;
          result.lead = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportLeadSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportLeadSetBuilder();
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

/// `discord`: the source leads, the channel here is read-only and mirrored; `bothy`: this server leads (22, 12.1).
class ImportLeadSetLeadEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'discord')
  static const ImportLeadSetLeadEnum discord = _$importLeadSetLeadEnum_discord;
  @BuiltValueEnumConst(wireName: r'bothy')
  static const ImportLeadSetLeadEnum bothy = _$importLeadSetLeadEnum_bothy;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportLeadSetLeadEnum unknownDefaultOpenApi =
      _$importLeadSetLeadEnum_unknownDefaultOpenApi;

  static Serializer<ImportLeadSetLeadEnum> get serializer =>
      _$importLeadSetLeadEnumSerializer;

  const ImportLeadSetLeadEnum._(String name) : super(name);

  static BuiltSet<ImportLeadSetLeadEnum> get values =>
      _$importLeadSetLeadEnumValues;
  static ImportLeadSetLeadEnum valueOf(String name) =>
      _$importLeadSetLeadEnumValueOf(name);
}
