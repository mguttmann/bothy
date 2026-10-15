//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/app_manifest_requested_grants_inner.dart';
import 'package:api_client/src/generated/model/command_register.dart';
import 'package:api_client/src/generated/model/app_manifest_delivery.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_manifest_incoming_webhooks_inner.dart';
import 'package:api_client/src/generated/model/app_manifest_scopes.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest.g.dart';

/// Schema `app_manifest.v1` (11, 3.1). Unknown members are an error. Beyond the schema the server applies the check rules of 11, 3.2: scopes from the catalogue, only subscribable events with the scope of their receive action and `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`, `requested_grants` at most `write` and at most the capability `upload` and never on a host, commands without reserved names and at most `command.max_per_principal`, and at most `app.manifest_max_bytes`.
///
/// Properties:
/// * [schemaVersion]
/// * [slug] - Unique in the instance; becomes the handle of the app principal.
/// * [name] - Becomes the display name of the app principal.
/// * [description]
/// * [publisher]
/// * [version] - SemVer 2.0.0 of the manifest (`apps.manifest_version`).
/// * [homepageUrl] - Absolute `https` address for display only; the server never loads it.
/// * [minApiVersion] - Lowest API version (major.minor) the app needs; not above the version of this server.
/// * [scopes]
/// * [events] - Durable event types (05, 6) the app may subscribe to; each needs the scope of its receive action and `events:subscribe`.
/// * [delivery]
/// * [commands] - Commands after 09, 10, without reserved names and without a target.
/// * [components] - Component types the app uses; only shown for the consent.
/// * [requestedGrants] - Grants the app asks for; the owner creates them per target with `createGrant`. Only `read` or `write`, at most the capability `upload`, never on a host.
/// * [incomingWebhooks]
/// * [dataEgress] - Plain text on which data leave the instance and where to; required with `events` or `interactions_url`.
@BuiltValue()
abstract class AppManifest implements Built<AppManifest, AppManifestBuilder> {
  @BuiltValueField(wireName: r'schema_version')
  AppManifestSchemaVersionEnum get schemaVersion;
  // enum schemaVersionEnum {  1,  };

  /// Unique in the instance; becomes the handle of the app principal.
  @BuiltValueField(wireName: r'slug')
  String get slug;

  /// Becomes the display name of the app principal.
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'publisher')
  String get publisher;

  /// SemVer 2.0.0 of the manifest (`apps.manifest_version`).
  @BuiltValueField(wireName: r'version')
  String get version;

  /// Absolute `https` address for display only; the server never loads it.
  @BuiltValueField(wireName: r'homepage_url')
  String? get homepageUrl;

  /// Lowest API version (major.minor) the app needs; not above the version of this server.
  @BuiltValueField(wireName: r'min_api_version')
  String get minApiVersion;

  @BuiltValueField(wireName: r'scopes')
  AppManifestScopes get scopes;

  /// Durable event types (05, 6) the app may subscribe to; each needs the scope of its receive action and `events:subscribe`.
  @BuiltValueField(wireName: r'events')
  BuiltList<String>? get events;

  @BuiltValueField(wireName: r'delivery')
  AppManifestDelivery? get delivery;

  /// Commands after 09, 10, without reserved names and without a target.
  @BuiltValueField(wireName: r'commands')
  BuiltList<CommandRegister>? get commands;

  /// Component types the app uses; only shown for the consent.
  @BuiltValueField(wireName: r'components')
  BuiltList<AppManifestComponentsEnum>? get components;
  // enum componentsEnum {  button,  select,  form,  };

  /// Grants the app asks for; the owner creates them per target with `createGrant`. Only `read` or `write`, at most the capability `upload`, never on a host.
  @BuiltValueField(wireName: r'requested_grants')
  BuiltList<AppManifestRequestedGrantsInner>? get requestedGrants;

  @BuiltValueField(wireName: r'incoming_webhooks')
  BuiltList<AppManifestIncomingWebhooksInner>? get incomingWebhooks;

  /// Plain text on which data leave the instance and where to; required with `events` or `interactions_url`.
  @BuiltValueField(wireName: r'data_egress')
  String? get dataEgress;

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
      specifiedType: const FullType(AppManifestSchemaVersionEnum),
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
    yield r'description';
    yield serializers.serialize(
      object.description,
      specifiedType: const FullType(String),
    );
    yield r'publisher';
    yield serializers.serialize(
      object.publisher,
      specifiedType: const FullType(String),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(String),
    );
    if (object.homepageUrl != null) {
      yield r'homepage_url';
      yield serializers.serialize(
        object.homepageUrl,
        specifiedType: const FullType(String),
      );
    }
    yield r'min_api_version';
    yield serializers.serialize(
      object.minApiVersion,
      specifiedType: const FullType(String),
    );
    yield r'scopes';
    yield serializers.serialize(
      object.scopes,
      specifiedType: const FullType(AppManifestScopes),
    );
    if (object.events != null) {
      yield r'events';
      yield serializers.serialize(
        object.events,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.delivery != null) {
      yield r'delivery';
      yield serializers.serialize(
        object.delivery,
        specifiedType: const FullType(AppManifestDelivery),
      );
    }
    if (object.commands != null) {
      yield r'commands';
      yield serializers.serialize(
        object.commands,
        specifiedType: const FullType(BuiltList, [FullType(CommandRegister)]),
      );
    }
    if (object.components != null) {
      yield r'components';
      yield serializers.serialize(
        object.components,
        specifiedType: const FullType(BuiltList, [
          FullType(AppManifestComponentsEnum),
        ]),
      );
    }
    if (object.requestedGrants != null) {
      yield r'requested_grants';
      yield serializers.serialize(
        object.requestedGrants,
        specifiedType: const FullType(BuiltList, [
          FullType(AppManifestRequestedGrantsInner),
        ]),
      );
    }
    if (object.incomingWebhooks != null) {
      yield r'incoming_webhooks';
      yield serializers.serialize(
        object.incomingWebhooks,
        specifiedType: const FullType(BuiltList, [
          FullType(AppManifestIncomingWebhooksInner),
        ]),
      );
    }
    if (object.dataEgress != null) {
      yield r'data_egress';
      yield serializers.serialize(
        object.dataEgress,
        specifiedType: const FullType(String),
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
            specifiedType: const FullType(AppManifestSchemaVersionEnum),
          ) as AppManifestSchemaVersionEnum;
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
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.description = valueDes;
          break;
        case r'publisher':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.publisher = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.version = valueDes;
          break;
        case r'homepage_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.homepageUrl = valueDes;
          break;
        case r'min_api_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.minApiVersion = valueDes;
          break;
        case r'scopes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppManifestScopes),
          ) as AppManifestScopes;
          result.scopes.replace(valueDes);
          break;
        case r'events':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.events.replace(valueDes);
          break;
        case r'delivery':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AppManifestDelivery),
          ) as AppManifestDelivery?;
          if (valueDes == null) continue;
          result.delivery.replace(valueDes);
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
        case r'components':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AppManifestComponentsEnum),
            ]),
          ) as BuiltList<AppManifestComponentsEnum>?;
          if (valueDes == null) continue;
          result.components.replace(valueDes);
          break;
        case r'requested_grants':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AppManifestRequestedGrantsInner),
            ]),
          ) as BuiltList<AppManifestRequestedGrantsInner>?;
          if (valueDes == null) continue;
          result.requestedGrants.replace(valueDes);
          break;
        case r'incoming_webhooks':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AppManifestIncomingWebhooksInner),
            ]),
          ) as BuiltList<AppManifestIncomingWebhooksInner>?;
          if (valueDes == null) continue;
          result.incomingWebhooks.replace(valueDes);
          break;
        case r'data_egress':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dataEgress = valueDes;
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

class AppManifestSchemaVersionEnum extends EnumClass {
  @BuiltValueEnumConst(wireNumber: 1)
  static const AppManifestSchemaVersionEnum number1 =
      _$appManifestSchemaVersionEnum_number1;
  @BuiltValueEnumConst(wireNumber: 11184809, fallback: true)
  static const AppManifestSchemaVersionEnum unknownDefaultOpenApi =
      _$appManifestSchemaVersionEnum_unknownDefaultOpenApi;

  static Serializer<AppManifestSchemaVersionEnum> get serializer =>
      _$appManifestSchemaVersionEnumSerializer;

  const AppManifestSchemaVersionEnum._(String name) : super(name);

  static BuiltSet<AppManifestSchemaVersionEnum> get values =>
      _$appManifestSchemaVersionEnumValues;
  static AppManifestSchemaVersionEnum valueOf(String name) =>
      _$appManifestSchemaVersionEnumValueOf(name);
}

class AppManifestComponentsEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'button')
  static const AppManifestComponentsEnum button =
      _$appManifestComponentsEnum_button;
  @BuiltValueEnumConst(wireName: r'select')
  static const AppManifestComponentsEnum select =
      _$appManifestComponentsEnum_select;
  @BuiltValueEnumConst(wireName: r'form')
  static const AppManifestComponentsEnum form =
      _$appManifestComponentsEnum_form;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AppManifestComponentsEnum unknownDefaultOpenApi =
      _$appManifestComponentsEnum_unknownDefaultOpenApi;

  static Serializer<AppManifestComponentsEnum> get serializer =>
      _$appManifestComponentsEnumSerializer;

  const AppManifestComponentsEnum._(String name) : super(name);

  static BuiltSet<AppManifestComponentsEnum> get values =>
      _$appManifestComponentsEnumValues;
  static AppManifestComponentsEnum valueOf(String name) =>
      _$appManifestComponentsEnumValueOf(name);
}
