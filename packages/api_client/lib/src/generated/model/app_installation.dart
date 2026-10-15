//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_installation.g.dart';

/// AppInstallation
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [grantedScopes]
/// * [eventSubscriptions]
/// * [state]
/// * [installedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [installedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class AppInstallation
    implements Built<AppInstallation, AppInstallationBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'granted_scopes')
  BuiltList<AppScope> get grantedScopes;

  @BuiltValueField(wireName: r'event_subscriptions')
  BuiltList<String> get eventSubscriptions;

  @BuiltValueField(wireName: r'state')
  AppInstallationStateEnum get state;
  // enum stateEnum {  active,  suspended,  uninstalled,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'installed_by')
  String? get installedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'installed_at')
  DateTime get installedAt;

  AppInstallation._();

  factory AppInstallation([void updates(AppInstallationBuilder b)]) =
      _$AppInstallation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppInstallationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppInstallation> get serializer =>
      _$AppInstallationSerializer();
}

class _$AppInstallationSerializer
    implements PrimitiveSerializer<AppInstallation> {
  @override
  final Iterable<Type> types = const [AppInstallation, _$AppInstallation];

  @override
  final String wireName = r'AppInstallation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppInstallation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'granted_scopes';
    yield serializers.serialize(
      object.grantedScopes,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
    yield r'event_subscriptions';
    yield serializers.serialize(
      object.eventSubscriptions,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(AppInstallationStateEnum),
    );
    if (object.installedBy != null) {
      yield r'installed_by';
      yield serializers.serialize(
        object.installedBy,
        specifiedType: const FullType(String),
      );
    }
    yield r'installed_at';
    yield serializers.serialize(
      object.installedAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppInstallation object, {
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
    required AppInstallationBuilder result,
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
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.eventSubscriptions.replace(valueDes);
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppInstallationStateEnum),
          ) as AppInstallationStateEnum;
          result.state = valueDes;
          break;
        case r'installed_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.installedBy = valueDes;
          break;
        case r'installed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.installedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppInstallation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppInstallationBuilder();
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

class AppInstallationStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const AppInstallationStateEnum active =
      _$appInstallationStateEnum_active;
  @BuiltValueEnumConst(wireName: r'suspended')
  static const AppInstallationStateEnum suspended =
      _$appInstallationStateEnum_suspended;
  @BuiltValueEnumConst(wireName: r'uninstalled')
  static const AppInstallationStateEnum uninstalled =
      _$appInstallationStateEnum_uninstalled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AppInstallationStateEnum unknownDefaultOpenApi =
      _$appInstallationStateEnum_unknownDefaultOpenApi;

  static Serializer<AppInstallationStateEnum> get serializer =>
      _$appInstallationStateEnumSerializer;

  const AppInstallationStateEnum._(String name) : super(name);

  static BuiltSet<AppInstallationStateEnum> get values =>
      _$appInstallationStateEnumValues;
  static AppInstallationStateEnum valueOf(String name) =>
      _$appInstallationStateEnumValueOf(name);
}
