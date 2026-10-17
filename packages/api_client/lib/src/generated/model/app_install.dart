//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_manifest.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_install.g.dart';

/// AppInstall
///
/// Properties:
/// * [manifest]
/// * [grantedScopes]
/// * [eventSubscriptions]
/// * [reason]
@BuiltValue()
abstract class AppInstall implements Built<AppInstall, AppInstallBuilder> {
  @BuiltValueField(wireName: r'manifest')
  AppManifest get manifest;

  @BuiltValueField(wireName: r'granted_scopes')
  BuiltList<AppScope> get grantedScopes;

  @BuiltValueField(wireName: r'event_subscriptions')
  BuiltList<String>? get eventSubscriptions;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  AppInstall._();

  factory AppInstall([void updates(AppInstallBuilder b)]) = _$AppInstall;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppInstallBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppInstall> get serializer => _$AppInstallSerializer();
}

class _$AppInstallSerializer implements PrimitiveSerializer<AppInstall> {
  @override
  final Iterable<Type> types = const [AppInstall, _$AppInstall];

  @override
  final String wireName = r'AppInstall';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppInstall object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'manifest';
    yield serializers.serialize(
      object.manifest,
      specifiedType: const FullType(AppManifest),
    );
    yield r'granted_scopes';
    yield serializers.serialize(
      object.grantedScopes,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
    if (object.eventSubscriptions != null) {
      yield r'event_subscriptions';
      yield serializers.serialize(
        object.eventSubscriptions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppInstall object, {
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
    required AppInstallBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'manifest':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppManifest),
          ) as AppManifest;
          result.manifest.replace(valueDes);
          break;
        case r'granted_scopes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
          ) as BuiltList<AppScope>;
          result.grantedScopes.replace(valueDes);
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
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppInstall deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppInstallBuilder();
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
