//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/import_source_discord_config_channels_inner.dart';
import 'package:api_client/src/generated/model/import_source_discord_config_spaces_inner.dart';
import 'package:api_client/src/generated/model/import_source_discord_config_users_inner.dart';
import 'package:api_client/src/generated/model/import_source_discord_config_defaults.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_discord_config.g.dart';

/// Mapping file of schema `import_source.discord.v1` as JSON (22, 5.2); the web console converts the YAML of the owner. It holds no secret.
///
/// Properties:
/// * [version]
/// * [guildId] - A Discord snowflake as a string of digits.
/// * [defaults]
/// * [spaces]
/// * [channels]
/// * [users]
@BuiltValue()
abstract class ImportSourceDiscordConfig
    implements
        Built<ImportSourceDiscordConfig, ImportSourceDiscordConfigBuilder> {
  @BuiltValueField(wireName: r'version')
  ImportSourceDiscordConfigVersionEnum get version;
  // enum versionEnum {  1,  };

  /// A Discord snowflake as a string of digits.
  @BuiltValueField(wireName: r'guild_id')
  String get guildId;

  @BuiltValueField(wireName: r'defaults')
  ImportSourceDiscordConfigDefaults get defaults;

  @BuiltValueField(wireName: r'spaces')
  BuiltList<ImportSourceDiscordConfigSpacesInner>? get spaces;

  @BuiltValueField(wireName: r'channels')
  BuiltList<ImportSourceDiscordConfigChannelsInner>? get channels;

  @BuiltValueField(wireName: r'users')
  BuiltList<ImportSourceDiscordConfigUsersInner>? get users;

  ImportSourceDiscordConfig._();

  factory ImportSourceDiscordConfig([
    void updates(ImportSourceDiscordConfigBuilder b),
  ]) = _$ImportSourceDiscordConfig;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceDiscordConfigBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceDiscordConfig> get serializer =>
      _$ImportSourceDiscordConfigSerializer();
}

class _$ImportSourceDiscordConfigSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfig> {
  @override
  final Iterable<Type> types = const [
    ImportSourceDiscordConfig,
    _$ImportSourceDiscordConfig,
  ];

  @override
  final String wireName = r'ImportSourceDiscordConfig';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceDiscordConfig object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(ImportSourceDiscordConfigVersionEnum),
    );
    yield r'guild_id';
    yield serializers.serialize(
      object.guildId,
      specifiedType: const FullType(String),
    );
    yield r'defaults';
    yield serializers.serialize(
      object.defaults,
      specifiedType: const FullType(ImportSourceDiscordConfigDefaults),
    );
    if (object.spaces != null) {
      yield r'spaces';
      yield serializers.serialize(
        object.spaces,
        specifiedType: const FullType(BuiltList, [
          FullType(ImportSourceDiscordConfigSpacesInner),
        ]),
      );
    }
    if (object.channels != null) {
      yield r'channels';
      yield serializers.serialize(
        object.channels,
        specifiedType: const FullType(BuiltList, [
          FullType(ImportSourceDiscordConfigChannelsInner),
        ]),
      );
    }
    if (object.users != null) {
      yield r'users';
      yield serializers.serialize(
        object.users,
        specifiedType: const FullType(BuiltList, [
          FullType(ImportSourceDiscordConfigUsersInner),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfig object, {
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
    required ImportSourceDiscordConfigBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportSourceDiscordConfigVersionEnum),
          ) as ImportSourceDiscordConfigVersionEnum;
          result.version = valueDes;
          break;
        case r'guild_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.guildId = valueDes;
          break;
        case r'defaults':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportSourceDiscordConfigDefaults),
          ) as ImportSourceDiscordConfigDefaults;
          result.defaults.replace(valueDes);
          break;
        case r'spaces':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ImportSourceDiscordConfigSpacesInner),
            ]),
          ) as BuiltList<ImportSourceDiscordConfigSpacesInner>?;
          if (valueDes == null) continue;
          result.spaces.replace(valueDes);
          break;
        case r'channels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ImportSourceDiscordConfigChannelsInner),
            ]),
          ) as BuiltList<ImportSourceDiscordConfigChannelsInner>?;
          if (valueDes == null) continue;
          result.channels.replace(valueDes);
          break;
        case r'users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ImportSourceDiscordConfigUsersInner),
            ]),
          ) as BuiltList<ImportSourceDiscordConfigUsersInner>?;
          if (valueDes == null) continue;
          result.users.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSourceDiscordConfig deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceDiscordConfigBuilder();
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

class ImportSourceDiscordConfigVersionEnum extends EnumClass {
  @BuiltValueEnumConst(wireNumber: 1)
  static const ImportSourceDiscordConfigVersionEnum number1 =
      _$importSourceDiscordConfigVersionEnum_number1;
  @BuiltValueEnumConst(wireNumber: 11184809, fallback: true)
  static const ImportSourceDiscordConfigVersionEnum unknownDefaultOpenApi =
      _$importSourceDiscordConfigVersionEnum_unknownDefaultOpenApi;

  static Serializer<ImportSourceDiscordConfigVersionEnum> get serializer =>
      _$importSourceDiscordConfigVersionEnumSerializer;

  const ImportSourceDiscordConfigVersionEnum._(String name) : super(name);

  static BuiltSet<ImportSourceDiscordConfigVersionEnum> get values =>
      _$importSourceDiscordConfigVersionEnumValues;
  static ImportSourceDiscordConfigVersionEnum valueOf(String name) =>
      _$importSourceDiscordConfigVersionEnumValueOf(name);
}
