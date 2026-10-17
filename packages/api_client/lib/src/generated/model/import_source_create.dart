//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/import_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_create.g.dart';

/// ImportSourceCreate
///
/// Properties:
/// * [kind]
/// * [name]
/// * [config] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class ImportSourceCreate
    implements Built<ImportSourceCreate, ImportSourceCreateBuilder> {
  @BuiltValueField(wireName: r'kind')
  ImportKind get kind;
  // enum kindEnum {  discord,  archive,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'config')
  BuiltMap<String, JsonObject?> get config;

  ImportSourceCreate._();

  factory ImportSourceCreate([void updates(ImportSourceCreateBuilder b)]) =
      _$ImportSourceCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceCreate> get serializer =>
      _$ImportSourceCreateSerializer();
}

class _$ImportSourceCreateSerializer
    implements PrimitiveSerializer<ImportSourceCreate> {
  @override
  final Iterable<Type> types = const [ImportSourceCreate, _$ImportSourceCreate];

  @override
  final String wireName = r'ImportSourceCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ImportKind),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'config';
    yield serializers.serialize(
      object.config,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceCreate object, {
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
    required ImportSourceCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportKind),
          ) as ImportKind;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.config.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSourceCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceCreateBuilder();
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
