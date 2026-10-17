//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/client_metric.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'client_metrics_batch.g.dart';

/// Anonymous client measurements (15, section 17). No contents, no IDs of threads, channels or people.
///
/// Properties:
/// * [platform]
/// * [appVersion]
/// * [density] - Mobile apps always send `touch` (01, section 11).
/// * [theme]
/// * [metrics]
@BuiltValue()
abstract class ClientMetricsBatch
    implements Built<ClientMetricsBatch, ClientMetricsBatchBuilder> {
  @BuiltValueField(wireName: r'platform')
  ClientMetricsBatchPlatformEnum get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  };

  @BuiltValueField(wireName: r'app_version')
  String get appVersion;

  /// Mobile apps always send `touch` (01, section 11).
  @BuiltValueField(wireName: r'density')
  ClientMetricsBatchDensityEnum? get density;
  // enum densityEnum {  comfortable,  compact,  touch,  };

  @BuiltValueField(wireName: r'theme')
  ClientMetricsBatchThemeEnum? get theme;
  // enum themeEnum {  light,  dark,  };

  @BuiltValueField(wireName: r'metrics')
  BuiltList<ClientMetric> get metrics;

  ClientMetricsBatch._();

  factory ClientMetricsBatch([void updates(ClientMetricsBatchBuilder b)]) =
      _$ClientMetricsBatch;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ClientMetricsBatchBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ClientMetricsBatch> get serializer =>
      _$ClientMetricsBatchSerializer();
}

class _$ClientMetricsBatchSerializer
    implements PrimitiveSerializer<ClientMetricsBatch> {
  @override
  final Iterable<Type> types = const [ClientMetricsBatch, _$ClientMetricsBatch];

  @override
  final String wireName = r'ClientMetricsBatch';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ClientMetricsBatch object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(ClientMetricsBatchPlatformEnum),
    );
    yield r'app_version';
    yield serializers.serialize(
      object.appVersion,
      specifiedType: const FullType(String),
    );
    if (object.density != null) {
      yield r'density';
      yield serializers.serialize(
        object.density,
        specifiedType: const FullType(ClientMetricsBatchDensityEnum),
      );
    }
    if (object.theme != null) {
      yield r'theme';
      yield serializers.serialize(
        object.theme,
        specifiedType: const FullType(ClientMetricsBatchThemeEnum),
      );
    }
    yield r'metrics';
    yield serializers.serialize(
      object.metrics,
      specifiedType: const FullType(BuiltList, [FullType(ClientMetric)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricsBatch object, {
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
    required ClientMetricsBatchBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ClientMetricsBatchPlatformEnum),
          ) as ClientMetricsBatchPlatformEnum;
          result.platform = valueDes;
          break;
        case r'app_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appVersion = valueDes;
          break;
        case r'density':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              ClientMetricsBatchDensityEnum,
            ),
          ) as ClientMetricsBatchDensityEnum?;
          if (valueDes == null) continue;
          result.density = valueDes;
          break;
        case r'theme':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ClientMetricsBatchThemeEnum),
          ) as ClientMetricsBatchThemeEnum?;
          if (valueDes == null) continue;
          result.theme = valueDes;
          break;
        case r'metrics':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ClientMetric)]),
          ) as BuiltList<ClientMetric>;
          result.metrics.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ClientMetricsBatch deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ClientMetricsBatchBuilder();
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

class ClientMetricsBatchPlatformEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'ios')
  static const ClientMetricsBatchPlatformEnum ios =
      _$clientMetricsBatchPlatformEnum_ios;
  @BuiltValueEnumConst(wireName: r'android')
  static const ClientMetricsBatchPlatformEnum android =
      _$clientMetricsBatchPlatformEnum_android;
  @BuiltValueEnumConst(wireName: r'macos')
  static const ClientMetricsBatchPlatformEnum macos =
      _$clientMetricsBatchPlatformEnum_macos;
  @BuiltValueEnumConst(wireName: r'windows')
  static const ClientMetricsBatchPlatformEnum windows =
      _$clientMetricsBatchPlatformEnum_windows;
  @BuiltValueEnumConst(wireName: r'web')
  static const ClientMetricsBatchPlatformEnum web =
      _$clientMetricsBatchPlatformEnum_web;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ClientMetricsBatchPlatformEnum unknownDefaultOpenApi =
      _$clientMetricsBatchPlatformEnum_unknownDefaultOpenApi;

  static Serializer<ClientMetricsBatchPlatformEnum> get serializer =>
      _$clientMetricsBatchPlatformEnumSerializer;

  const ClientMetricsBatchPlatformEnum._(String name) : super(name);

  static BuiltSet<ClientMetricsBatchPlatformEnum> get values =>
      _$clientMetricsBatchPlatformEnumValues;
  static ClientMetricsBatchPlatformEnum valueOf(String name) =>
      _$clientMetricsBatchPlatformEnumValueOf(name);
}

/// Mobile apps always send `touch` (01, section 11).
class ClientMetricsBatchDensityEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'comfortable')
  static const ClientMetricsBatchDensityEnum comfortable =
      _$clientMetricsBatchDensityEnum_comfortable;
  @BuiltValueEnumConst(wireName: r'compact')
  static const ClientMetricsBatchDensityEnum compact =
      _$clientMetricsBatchDensityEnum_compact;
  @BuiltValueEnumConst(wireName: r'touch')
  static const ClientMetricsBatchDensityEnum touch =
      _$clientMetricsBatchDensityEnum_touch;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ClientMetricsBatchDensityEnum unknownDefaultOpenApi =
      _$clientMetricsBatchDensityEnum_unknownDefaultOpenApi;

  static Serializer<ClientMetricsBatchDensityEnum> get serializer =>
      _$clientMetricsBatchDensityEnumSerializer;

  const ClientMetricsBatchDensityEnum._(String name) : super(name);

  static BuiltSet<ClientMetricsBatchDensityEnum> get values =>
      _$clientMetricsBatchDensityEnumValues;
  static ClientMetricsBatchDensityEnum valueOf(String name) =>
      _$clientMetricsBatchDensityEnumValueOf(name);
}

class ClientMetricsBatchThemeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'light')
  static const ClientMetricsBatchThemeEnum light =
      _$clientMetricsBatchThemeEnum_light;
  @BuiltValueEnumConst(wireName: r'dark')
  static const ClientMetricsBatchThemeEnum dark =
      _$clientMetricsBatchThemeEnum_dark;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ClientMetricsBatchThemeEnum unknownDefaultOpenApi =
      _$clientMetricsBatchThemeEnum_unknownDefaultOpenApi;

  static Serializer<ClientMetricsBatchThemeEnum> get serializer =>
      _$clientMetricsBatchThemeEnumSerializer;

  const ClientMetricsBatchThemeEnum._(String name) : super(name);

  static BuiltSet<ClientMetricsBatchThemeEnum> get values =>
      _$clientMetricsBatchThemeEnumValues;
  static ClientMetricsBatchThemeEnum valueOf(String name) =>
      _$clientMetricsBatchThemeEnumValueOf(name);
}
