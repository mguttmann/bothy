//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source_discord_config_users_inner.g.dart';

/// ImportSourceDiscordConfigUsersInner
///
/// Properties:
/// * [discordUser] - A Discord snowflake as a string of digits.
/// * [target] - `user:<handle>`, `agent:<handle>` or `placeholder`.
@BuiltValue()
abstract class ImportSourceDiscordConfigUsersInner
    implements
        Built<
          ImportSourceDiscordConfigUsersInner,
          ImportSourceDiscordConfigUsersInnerBuilder
        > {
  /// A Discord snowflake as a string of digits.
  @BuiltValueField(wireName: r'discord_user')
  String get discordUser;

  /// `user:<handle>`, `agent:<handle>` or `placeholder`.
  @BuiltValueField(wireName: r'target')
  String get target;

  ImportSourceDiscordConfigUsersInner._();

  factory ImportSourceDiscordConfigUsersInner([
    void updates(ImportSourceDiscordConfigUsersInnerBuilder b),
  ]) = _$ImportSourceDiscordConfigUsersInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceDiscordConfigUsersInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSourceDiscordConfigUsersInner> get serializer =>
      _$ImportSourceDiscordConfigUsersInnerSerializer();
}

class _$ImportSourceDiscordConfigUsersInnerSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfigUsersInner> {
  @override
  final Iterable<Type> types = const [
    ImportSourceDiscordConfigUsersInner,
    _$ImportSourceDiscordConfigUsersInner,
  ];

  @override
  final String wireName = r'ImportSourceDiscordConfigUsersInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSourceDiscordConfigUsersInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'discord_user';
    yield serializers.serialize(
      object.discordUser,
      specifiedType: const FullType(String),
    );
    yield r'target';
    yield serializers.serialize(
      object.target,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigUsersInner object, {
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
    required ImportSourceDiscordConfigUsersInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'discord_user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.discordUser = valueDes;
          break;
        case r'target':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
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
  ImportSourceDiscordConfigUsersInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceDiscordConfigUsersInnerBuilder();
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
