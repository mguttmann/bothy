//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_discord_config_channels_inner.g.dart';

/// ImportSourceDiscordConfigChannelsInner
///
/// Properties:
/// * [discordChannel] - A Discord snowflake as a string of digits.
/// * [target] - Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server checks the form and reports a violation at the pointer of the member.
/// * [plannedOperator] - `agent:<handle>` of an active agent or `none` (default).
/// * [lead]
@BuiltValue()
abstract class ImportSourceDiscordConfigChannelsInner
    implements
        Built<
          ImportSourceDiscordConfigChannelsInner,
          ImportSourceDiscordConfigChannelsInnerBuilder
        > {
  /// A Discord snowflake as a string of digits.
  @BuiltValueField(wireName: r'discord_channel')
  String get discordChannel;

  /// Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server checks the form and reports a violation at the pointer of the member.
  @BuiltValueField(wireName: r'target')
  JsonObject? get target;

  /// `agent:<handle>` of an active agent or `none` (default).
  @BuiltValueField(wireName: r'planned_operator')
  String? get plannedOperator;

  @BuiltValueField(wireName: r'lead')
  ImportSourceDiscordConfigChannelsInnerLeadEnum? get lead;
  // enum leadEnum {  discord,  bothy,  };

  ImportSourceDiscordConfigChannelsInner._();

  factory ImportSourceDiscordConfigChannelsInner([
    void updates(ImportSourceDiscordConfigChannelsInnerBuilder b),
  ]) = _$ImportSourceDiscordConfigChannelsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceDiscordConfigChannelsInnerBuilder b) => b
    ..lead = ImportSourceDiscordConfigChannelsInnerLeadEnum.valueOf('discord');

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceDiscordConfigChannelsInner> get serializer =>
      _$ImportSourceDiscordConfigChannelsInnerSerializer();
}

class _$ImportSourceDiscordConfigChannelsInnerSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfigChannelsInner> {
  @override
  final Iterable<Type> types = const [
    ImportSourceDiscordConfigChannelsInner,
    _$ImportSourceDiscordConfigChannelsInner,
  ];

  @override
  final String wireName = r'ImportSourceDiscordConfigChannelsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceDiscordConfigChannelsInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'discord_channel';
    yield serializers.serialize(
      object.discordChannel,
      specifiedType: const FullType(String),
    );
    yield r'target';
    yield object.target == null
        ? null
        : serializers.serialize(
            object.target,
            specifiedType: const FullType.nullable(JsonObject),
          );
    if (object.plannedOperator != null) {
      yield r'planned_operator';
      yield serializers.serialize(
        object.plannedOperator,
        specifiedType: const FullType(String),
      );
    }
    if (object.lead != null) {
      yield r'lead';
      yield serializers.serialize(
        object.lead,
        specifiedType: const FullType(
          ImportSourceDiscordConfigChannelsInnerLeadEnum,
        ),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigChannelsInner object, {
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
    required ImportSourceDiscordConfigChannelsInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'discord_channel':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.discordChannel = valueDes;
          break;
        case r'target':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(JsonObject),
          ) as JsonObject?;
          if (valueDes == null) continue;
          result.target = valueDes;
          break;
        case r'planned_operator':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.plannedOperator = valueDes;
          break;
        case r'lead':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              ImportSourceDiscordConfigChannelsInnerLeadEnum,
            ),
          ) as ImportSourceDiscordConfigChannelsInnerLeadEnum?;
          if (valueDes == null) continue;
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
  ImportSourceDiscordConfigChannelsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceDiscordConfigChannelsInnerBuilder();
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

class ImportSourceDiscordConfigChannelsInnerLeadEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'discord')
  static const ImportSourceDiscordConfigChannelsInnerLeadEnum discord =
      _$importSourceDiscordConfigChannelsInnerLeadEnum_discord;
  @BuiltValueEnumConst(wireName: r'bothy')
  static const ImportSourceDiscordConfigChannelsInnerLeadEnum bothy =
      _$importSourceDiscordConfigChannelsInnerLeadEnum_bothy;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportSourceDiscordConfigChannelsInnerLeadEnum
  unknownDefaultOpenApi =
      _$importSourceDiscordConfigChannelsInnerLeadEnum_unknownDefaultOpenApi;

  static Serializer<ImportSourceDiscordConfigChannelsInnerLeadEnum>
  get serializer => _$importSourceDiscordConfigChannelsInnerLeadEnumSerializer;

  const ImportSourceDiscordConfigChannelsInnerLeadEnum._(String name)
    : super(name);

  static BuiltSet<ImportSourceDiscordConfigChannelsInnerLeadEnum> get values =>
      _$importSourceDiscordConfigChannelsInnerLeadEnumValues;
  static ImportSourceDiscordConfigChannelsInnerLeadEnum valueOf(String name) =>
      _$importSourceDiscordConfigChannelsInnerLeadEnumValueOf(name);
}
