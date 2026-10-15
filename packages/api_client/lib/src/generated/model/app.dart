//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/app_manifest.dart';
import 'package:api_client/src/generated/model/app_installation.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app.g.dart';

/// App
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [slug]
/// * [displayName]
/// * [manifest]
/// * [manifestVersion]
/// * [installation]
@BuiltValue()
abstract class App implements Built<App, AppBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'slug')
  String get slug;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  @BuiltValueField(wireName: r'manifest')
  AppManifest get manifest;

  @BuiltValueField(wireName: r'manifest_version')
  String get manifestVersion;

  @BuiltValueField(wireName: r'installation')
  AppInstallation get installation;

  App._();

  factory App([void updates(AppBuilder b)]) = _$App;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<App> get serializer => _$AppSerializer();
}

class _$AppSerializer implements PrimitiveSerializer<App> {
  @override
  final Iterable<Type> types = const [App, _$App];

  @override
  final String wireName = r'App';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    App object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'slug';
    yield serializers.serialize(
      object.slug,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    yield r'manifest';
    yield serializers.serialize(
      object.manifest,
      specifiedType: const FullType(AppManifest),
    );
    yield r'manifest_version';
    yield serializers.serialize(
      object.manifestVersion,
      specifiedType: const FullType(String),
    );
    yield r'installation';
    yield serializers.serialize(
      object.installation,
      specifiedType: const FullType(AppInstallation),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    App object, {
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
    required AppBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'slug':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.slug = valueDes;
          break;
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'manifest':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppManifest),
          ) as AppManifest;
          result.manifest.replace(valueDes);
          break;
        case r'manifest_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.manifestVersion = valueDes;
          break;
        case r'installation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppInstallation),
          ) as AppInstallation;
          result.installation.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  App deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppBuilder();
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
