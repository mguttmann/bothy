//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_discord_config_spaces_inner.g.dart';

/// ImportSourceDiscordConfigSpacesInner
///
/// Properties:
/// * [discordCategory] - A Discord snowflake as a string of digits.
/// * [target] - Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server checks the form and reports a violation at the pointer of the member.
@BuiltValue()
abstract class ImportSourceDiscordConfigSpacesInner
    implements
        Built<
          ImportSourceDiscordConfigSpacesInner,
          ImportSourceDiscordConfigSpacesInnerBuilder
        > {
  /// A Discord snowflake as a string of digits.
  @BuiltValueField(wireName: r'discord_category')
  String get discordCategory;

  /// Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server checks the form and reports a violation at the pointer of the member.
  @BuiltValueField(wireName: r'target')
  JsonObject? get target;

  ImportSourceDiscordConfigSpacesInner._();

  factory ImportSourceDiscordConfigSpacesInner([
    void updates(ImportSourceDiscordConfigSpacesInnerBuilder b),
  ]) = _$ImportSourceDiscordConfigSpacesInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceDiscordConfigSpacesInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceDiscordConfigSpacesInner> get serializer =>
      _$ImportSourceDiscordConfigSpacesInnerSerializer();
}

class _$ImportSourceDiscordConfigSpacesInnerSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfigSpacesInner> {
  @override
  final Iterable<Type> types = const [
    ImportSourceDiscordConfigSpacesInner,
    _$ImportSourceDiscordConfigSpacesInner,
  ];

  @override
  final String wireName = r'ImportSourceDiscordConfigSpacesInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceDiscordConfigSpacesInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'discord_category';
    yield serializers.serialize(
      object.discordCategory,
      specifiedType: const FullType(String),
    );
    yield r'target';
    yield object.target == null
        ? null
        : serializers.serialize(
            object.target,
            specifiedType: const FullType.nullable(JsonObject),
          );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigSpacesInner object, {
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
    required ImportSourceDiscordConfigSpacesInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'discord_category':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.discordCategory = valueDes;
          break;
        case r'target':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(JsonObject),
          ) as JsonObject?;
          if (valueDes == null) continue;
          result.target = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSourceDiscordConfigSpacesInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceDiscordConfigSpacesInnerBuilder();
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
