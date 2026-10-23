//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_user_data.g.dart';

/// `data` of a `user` item (origin key `user:<user_id>`). The target comes from the stored mapping and its `defaults`; a webhook author is always a placeholder (22, 5.3).
///
/// Properties:
/// * [discordKind]
/// * [username]
/// * [globalName]
@BuiltValue()
abstract class ImportUserData
    implements Built<ImportUserData, ImportUserDataBuilder> {
  @BuiltValueField(wireName: r'discord_kind')
  ImportUserDataDiscordKindEnum get discordKind;
  // enum discordKindEnum {  user,  bot,  webhook,  };

  @BuiltValueField(wireName: r'username')
  String get username;

  @BuiltValueField(wireName: r'global_name')
  String? get globalName;

  ImportUserData._();

  factory ImportUserData([void updates(ImportUserDataBuilder b)]) =
      _$ImportUserData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportUserDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportUserData> get serializer =>
      _$ImportUserDataSerializer();
}

class _$ImportUserDataSerializer
    implements PrimitiveSerializer<ImportUserData> {
  @override
  final Iterable<Type> types = const [ImportUserData, _$ImportUserData];

  @override
  final String wireName = r'ImportUserData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportUserData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'discord_kind';
    yield serializers.serialize(
      object.discordKind,
      specifiedType: const FullType(ImportUserDataDiscordKindEnum),
    );
    yield r'username';
    yield serializers.serialize(
      object.username,
      specifiedType: const FullType(String),
    );
    if (object.globalName != null) {
      yield r'global_name';
      yield serializers.serialize(
        object.globalName,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportUserData object, {
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
    required ImportUserDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'discord_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportUserDataDiscordKindEnum),
          ) as ImportUserDataDiscordKindEnum;
          result.discordKind = valueDes;
          break;
        case r'username':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.username = valueDes;
          break;
        case r'global_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.globalName = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportUserData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportUserDataBuilder();
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

class ImportUserDataDiscordKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user')
  static const ImportUserDataDiscordKindEnum user =
      _$importUserDataDiscordKindEnum_user;
  @BuiltValueEnumConst(wireName: r'bot')
  static const ImportUserDataDiscordKindEnum bot =
      _$importUserDataDiscordKindEnum_bot;
  @BuiltValueEnumConst(wireName: r'webhook')
  static const ImportUserDataDiscordKindEnum webhook =
      _$importUserDataDiscordKindEnum_webhook;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportUserDataDiscordKindEnum unknownDefaultOpenApi =
      _$importUserDataDiscordKindEnum_unknownDefaultOpenApi;

  static Serializer<ImportUserDataDiscordKindEnum> get serializer =>
      _$importUserDataDiscordKindEnumSerializer;

  const ImportUserDataDiscordKindEnum._(String name) : super(name);

  static BuiltSet<ImportUserDataDiscordKindEnum> get values =>
      _$importUserDataDiscordKindEnumValues;
  static ImportUserDataDiscordKindEnum valueOf(String name) =>
      _$importUserDataDiscordKindEnumValueOf(name);
}
