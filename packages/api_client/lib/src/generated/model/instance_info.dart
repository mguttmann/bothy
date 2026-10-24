//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/branding.dart';
import 'package:api_client/src/generated/model/instance_limits.dart';
import 'package:api_client/src/generated/model/evaluated_flag.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'instance_info.g.dart';

/// Instance view for an authenticated principal.
///
/// Properties:
/// * [instanceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [branding]
/// * [localeDefault]
/// * [specVersion]
/// * [features] - Feature flags evaluated for the caller.
/// * [limits]
/// * [telemetryDashboardUrl] - Link of the administration to the telemetry backend, from the configuration key `telemetry.dashboard_url` (17 8). Present only for owners and only when the key is set, because the address names internal infrastructure; clients open it in the system browser or a new tab, never in a WebView or iframe.
/// * [statsTimeZone] - IANA name of the time zone in which the day, week and month buckets of the statistics operations start (`stats.time_zone`, 17 8); clients align the ranges of their statistics queries to its calendar days.
@BuiltValue()
abstract class InstanceInfo
    implements Built<InstanceInfo, InstanceInfoBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'instance_id')
  String get instanceId;

  @BuiltValueField(wireName: r'branding')
  Branding get branding;

  @BuiltValueField(wireName: r'locale_default')
  String get localeDefault;

  @BuiltValueField(wireName: r'spec_version')
  String get specVersion;

  /// Feature flags evaluated for the caller.
  @BuiltValueField(wireName: r'features')
  BuiltList<EvaluatedFlag> get features;

  @BuiltValueField(wireName: r'limits')
  InstanceLimits get limits;

  /// Link of the administration to the telemetry backend, from the configuration key `telemetry.dashboard_url` (17 8). Present only for owners and only when the key is set, because the address names internal infrastructure; clients open it in the system browser or a new tab, never in a WebView or iframe.
  @BuiltValueField(wireName: r'telemetry_dashboard_url')
  String? get telemetryDashboardUrl;

  /// IANA name of the time zone in which the day, week and month buckets of the statistics operations start (`stats.time_zone`, 17 8); clients align the ranges of their statistics queries to its calendar days.
  @BuiltValueField(wireName: r'stats_time_zone')
  String? get statsTimeZone;

  InstanceInfo._();

  factory InstanceInfo([void updates(InstanceInfoBuilder b)]) = _$InstanceInfo;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InstanceInfoBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InstanceInfo> get serializer => _$InstanceInfoSerializer();
}

class _$InstanceInfoSerializer implements PrimitiveSerializer<InstanceInfo> {
  @override
  final Iterable<Type> types = const [InstanceInfo, _$InstanceInfo];

  @override
  final String wireName = r'InstanceInfo';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InstanceInfo object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'instance_id';
    yield serializers.serialize(
      object.instanceId,
      specifiedType: const FullType(String),
    );
    yield r'branding';
    yield serializers.serialize(
      object.branding,
      specifiedType: const FullType(Branding),
    );
    yield r'locale_default';
    yield serializers.serialize(
      object.localeDefault,
      specifiedType: const FullType(String),
    );
    yield r'spec_version';
    yield serializers.serialize(
      object.specVersion,
      specifiedType: const FullType(String),
    );
    yield r'features';
    yield serializers.serialize(
      object.features,
      specifiedType: const FullType(BuiltList, [FullType(EvaluatedFlag)]),
    );
    yield r'limits';
    yield serializers.serialize(
      object.limits,
      specifiedType: const FullType(InstanceLimits),
    );
    if (object.telemetryDashboardUrl != null) {
      yield r'telemetry_dashboard_url';
      yield serializers.serialize(
        object.telemetryDashboardUrl,
        specifiedType: const FullType(String),
      );
    }
    if (object.statsTimeZone != null) {
      yield r'stats_time_zone';
      yield serializers.serialize(
        object.statsTimeZone,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    InstanceInfo object, {
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
    required InstanceInfoBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'instance_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.instanceId = valueDes;
          break;
        case r'branding':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Branding),
          ) as Branding;
          result.branding.replace(valueDes);
          break;
        case r'locale_default':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.localeDefault = valueDes;
          break;
        case r'spec_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.specVersion = valueDes;
          break;
        case r'features':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(EvaluatedFlag)]),
          ) as BuiltList<EvaluatedFlag>;
          result.features.replace(valueDes);
          break;
        case r'limits':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(InstanceLimits),
          ) as InstanceLimits;
          result.limits.replace(valueDes);
          break;
        case r'telemetry_dashboard_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.telemetryDashboardUrl = valueDes;
          break;
        case r'stats_time_zone':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.statsTimeZone = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InstanceInfo deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InstanceInfoBuilder();
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
