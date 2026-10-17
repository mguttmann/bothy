//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command_register.dart';
import 'package:api_client/src/generated/model/webhook_subscription_create.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest.g.dart';

/// Schema `app_manifest.v1`.
///
/// Properties:
/// * [schemaVersion]
/// * [slug]
/// * [name]
/// * [version] - SemVer of the manifest.
/// * [description]
/// * [publisher]
/// * [homepageUrl]
/// * [scopes]
/// * [eventSubscriptions]
/// * [commands]
/// * [webhooks]
@BuiltValue()
abstract class AppManifest implements Built<AppManifest, AppManifestBuilder> {
  @BuiltValueField(wireName: r'schema_version')
  int get schemaVersion;

  @BuiltValueField(wireName: r'slug')
  String get slug;

  @BuiltValueField(wireName: r'name')
  String get name;

  /// SemVer of the manifest.
  @BuiltValueField(wireName: r'version')
  String get version;

  @BuiltValueField(wireName: r'description')
  String? get description;

  @BuiltValueField(wireName: r'publisher')
  String? get publisher;

  @BuiltValueField(wireName: r'homepage_url')
  String? get homepageUrl;

  @BuiltValueField(wireName: r'scopes')
  BuiltList<AppScope> get scopes;

  @BuiltValueField(wireName: r'event_subscriptions')
  BuiltList<String>? get eventSubscriptions;

  @BuiltValueField(wireName: r'commands')
  BuiltList<CommandRegister>? get commands;

  @BuiltValueField(wireName: r'webhooks')
  BuiltList<WebhookSubscriptionCreate>? get webhooks;

  AppManifest._();

  factory AppManifest([void updates(AppManifestBuilder b)]) = _$AppManifest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppManifestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppManifest> get serializer => _$AppManifestSerializer();
}

class _$AppManifestSerializer implements PrimitiveSerializer<AppManifest> {
  @override
  final Iterable<Type> types = const [AppManifest, _$AppManifest];

  @override
  final String wireName = r'AppManifest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppManifest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'schema_version';
    yield serializers.serialize(
      object.schemaVersion,
      specifiedType: const FullType(int),
    );
    yield r'slug';
    yield serializers.serialize(
      object.slug,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(String),
    );
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.publisher != null) {
      yield r'publisher';
      yield serializers.serialize(
        object.publisher,
        specifiedType: const FullType(String),
      );
    }
    if (object.homepageUrl != null) {
      yield r'homepage_url';
      yield serializers.serialize(
        object.homepageUrl,
        specifiedType: const FullType(String),
      );
    }
    yield r'scopes';
    yield serializers.serialize(
      object.scopes,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
    if (object.eventSubscriptions != null) {
      yield r'event_subscriptions';
      yield serializers.serialize(
        object.eventSubscriptions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.commands != null) {
      yield r'commands';
      yield serializers.serialize(
        object.commands,
        specifiedType: const FullType(BuiltList, [FullType(CommandRegister)]),
      );
    }
    if (object.webhooks != null) {
      yield r'webhooks';
      yield serializers.serialize(
        object.webhooks,
        specifiedType: const FullType(BuiltList, [
          FullType(WebhookSubscriptionCreate),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AppManifest object, {
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
    required AppManifestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'schema_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.schemaVersion = valueDes;
          break;
        case r'slug':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.slug = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.version = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'publisher':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.publisher = valueDes;
          break;
        case r'homepage_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.homepageUrl = valueDes;
          break;
        case r'scopes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
          ) as BuiltList<AppScope>;
          result.scopes.replace(valueDes);
          break;
        case r'event_subscriptions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.eventSubscriptions.replace(valueDes);
          break;
        case r'commands':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandRegister),
            ]),
          ) as BuiltList<CommandRegister>?;
          if (valueDes == null) continue;
          result.commands.replace(valueDes);
          break;
        case r'webhooks':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(WebhookSubscriptionCreate),
            ]),
          ) as BuiltList<WebhookSubscriptionCreate>?;
          if (valueDes == null) continue;
          result.webhooks.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppManifest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppManifestBuilder();
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
