//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_discord_config_defaults.g.dart';

/// ImportSourceDiscordConfigDefaults
///
/// Properties:
/// * [unmappedHuman]
/// * [unmappedBot]
/// * [uncategorizedSpace] - `skip` or the `discord_category` of an entry in `spaces`.
@BuiltValue()
abstract class ImportSourceDiscordConfigDefaults
    implements
        Built<
          ImportSourceDiscordConfigDefaults,
          ImportSourceDiscordConfigDefaultsBuilder
        > {
  @BuiltValueField(wireName: r'unmapped_human')
  ImportSourceDiscordConfigDefaultsUnmappedHumanEnum get unmappedHuman;
  // enum unmappedHumanEnum {  placeholder,  skip,  };

  @BuiltValueField(wireName: r'unmapped_bot')
  ImportSourceDiscordConfigDefaultsUnmappedBotEnum get unmappedBot;
  // enum unmappedBotEnum {  placeholder,  skip,  };

  /// `skip` or the `discord_category` of an entry in `spaces`.
  @BuiltValueField(wireName: r'uncategorized_space')
  String get uncategorizedSpace;

  ImportSourceDiscordConfigDefaults._();

  factory ImportSourceDiscordConfigDefaults([
    void updates(ImportSourceDiscordConfigDefaultsBuilder b),
  ]) = _$ImportSourceDiscordConfigDefaults;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceDiscordConfigDefaultsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceDiscordConfigDefaults> get serializer =>
      _$ImportSourceDiscordConfigDefaultsSerializer();
}

class _$ImportSourceDiscordConfigDefaultsSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfigDefaults> {
  @override
  final Iterable<Type> types = const [
    ImportSourceDiscordConfigDefaults,
    _$ImportSourceDiscordConfigDefaults,
  ];

  @override
  final String wireName = r'ImportSourceDiscordConfigDefaults';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceDiscordConfigDefaults object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'unmapped_human';
    yield serializers.serialize(
      object.unmappedHuman,
      specifiedType: const FullType(
        ImportSourceDiscordConfigDefaultsUnmappedHumanEnum,
      ),
    );
    yield r'unmapped_bot';
    yield serializers.serialize(
      object.unmappedBot,
      specifiedType: const FullType(
        ImportSourceDiscordConfigDefaultsUnmappedBotEnum,
      ),
    );
    yield r'uncategorized_space';
    yield serializers.serialize(
      object.uncategorizedSpace,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigDefaults object, {
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
    required ImportSourceDiscordConfigDefaultsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'unmapped_human':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              ImportSourceDiscordConfigDefaultsUnmappedHumanEnum,
            ),
          ) as ImportSourceDiscordConfigDefaultsUnmappedHumanEnum;
          result.unmappedHuman = valueDes;
          break;
        case r'unmapped_bot':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              ImportSourceDiscordConfigDefaultsUnmappedBotEnum,
            ),
          ) as ImportSourceDiscordConfigDefaultsUnmappedBotEnum;
          result.unmappedBot = valueDes;
          break;
        case r'uncategorized_space':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.uncategorizedSpace = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSourceDiscordConfigDefaults deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceDiscordConfigDefaultsBuilder();
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

class ImportSourceDiscordConfigDefaultsUnmappedHumanEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'placeholder')
  static const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum placeholder =
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_placeholder;
  @BuiltValueEnumConst(wireName: r'skip')
  static const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum skip =
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_skip;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
  unknownDefaultOpenApi =
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_unknownDefaultOpenApi;

  static Serializer<ImportSourceDiscordConfigDefaultsUnmappedHumanEnum>
  get serializer =>
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnumSerializer;

  const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum._(String name)
    : super(name);

  static BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedHumanEnum>
  get values => _$importSourceDiscordConfigDefaultsUnmappedHumanEnumValues;
  static ImportSourceDiscordConfigDefaultsUnmappedHumanEnum valueOf(
    String name,
  ) => _$importSourceDiscordConfigDefaultsUnmappedHumanEnumValueOf(name);
}

class ImportSourceDiscordConfigDefaultsUnmappedBotEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'placeholder')
  static const ImportSourceDiscordConfigDefaultsUnmappedBotEnum placeholder =
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_placeholder;
  @BuiltValueEnumConst(wireName: r'skip')
  static const ImportSourceDiscordConfigDefaultsUnmappedBotEnum skip =
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_skip;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportSourceDiscordConfigDefaultsUnmappedBotEnum
  unknownDefaultOpenApi =
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_unknownDefaultOpenApi;

  static Serializer<ImportSourceDiscordConfigDefaultsUnmappedBotEnum>
  get serializer =>
      _$importSourceDiscordConfigDefaultsUnmappedBotEnumSerializer;

  const ImportSourceDiscordConfigDefaultsUnmappedBotEnum._(String name)
    : super(name);

  static BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedBotEnum>
  get values => _$importSourceDiscordConfigDefaultsUnmappedBotEnumValues;
  static ImportSourceDiscordConfigDefaultsUnmappedBotEnum valueOf(
    String name,
  ) => _$importSourceDiscordConfigDefaultsUnmappedBotEnumValueOf(name);
}
