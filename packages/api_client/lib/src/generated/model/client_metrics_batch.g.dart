// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'client_metrics_batch.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnum_ios =
    const ClientMetricsBatchPlatformEnum._('ios');
const ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnum_android =
    const ClientMetricsBatchPlatformEnum._('android');
const ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnum_macos =
    const ClientMetricsBatchPlatformEnum._('macos');
const ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnum_windows =
    const ClientMetricsBatchPlatformEnum._('windows');
const ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnum_web =
    const ClientMetricsBatchPlatformEnum._('web');
const ClientMetricsBatchPlatformEnum
_$clientMetricsBatchPlatformEnum_unknownDefaultOpenApi =
    const ClientMetricsBatchPlatformEnum._('unknownDefaultOpenApi');

ClientMetricsBatchPlatformEnum _$clientMetricsBatchPlatformEnumValueOf(
  String name,
) {
  switch (name) {
    case 'ios':
      return _$clientMetricsBatchPlatformEnum_ios;
    case 'android':
      return _$clientMetricsBatchPlatformEnum_android;
    case 'macos':
      return _$clientMetricsBatchPlatformEnum_macos;
    case 'windows':
      return _$clientMetricsBatchPlatformEnum_windows;
    case 'web':
      return _$clientMetricsBatchPlatformEnum_web;
    case 'unknownDefaultOpenApi':
      return _$clientMetricsBatchPlatformEnum_unknownDefaultOpenApi;
    default:
      return _$clientMetricsBatchPlatformEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ClientMetricsBatchPlatformEnum>
_$clientMetricsBatchPlatformEnumValues =
    BuiltSet<ClientMetricsBatchPlatformEnum>(
      const <ClientMetricsBatchPlatformEnum>[
        _$clientMetricsBatchPlatformEnum_ios,
        _$clientMetricsBatchPlatformEnum_android,
        _$clientMetricsBatchPlatformEnum_macos,
        _$clientMetricsBatchPlatformEnum_windows,
        _$clientMetricsBatchPlatformEnum_web,
        _$clientMetricsBatchPlatformEnum_unknownDefaultOpenApi,
      ],
    );

const ClientMetricsBatchDensityEnum
_$clientMetricsBatchDensityEnum_comfortable =
    const ClientMetricsBatchDensityEnum._('comfortable');
const ClientMetricsBatchDensityEnum _$clientMetricsBatchDensityEnum_compact =
    const ClientMetricsBatchDensityEnum._('compact');
const ClientMetricsBatchDensityEnum _$clientMetricsBatchDensityEnum_touch =
    const ClientMetricsBatchDensityEnum._('touch');
const ClientMetricsBatchDensityEnum
_$clientMetricsBatchDensityEnum_unknownDefaultOpenApi =
    const ClientMetricsBatchDensityEnum._('unknownDefaultOpenApi');

ClientMetricsBatchDensityEnum _$clientMetricsBatchDensityEnumValueOf(
  String name,
) {
  switch (name) {
    case 'comfortable':
      return _$clientMetricsBatchDensityEnum_comfortable;
    case 'compact':
      return _$clientMetricsBatchDensityEnum_compact;
    case 'touch':
      return _$clientMetricsBatchDensityEnum_touch;
    case 'unknownDefaultOpenApi':
      return _$clientMetricsBatchDensityEnum_unknownDefaultOpenApi;
    default:
      return _$clientMetricsBatchDensityEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ClientMetricsBatchDensityEnum>
_$clientMetricsBatchDensityEnumValues = BuiltSet<ClientMetricsBatchDensityEnum>(
  const <ClientMetricsBatchDensityEnum>[
    _$clientMetricsBatchDensityEnum_comfortable,
    _$clientMetricsBatchDensityEnum_compact,
    _$clientMetricsBatchDensityEnum_touch,
    _$clientMetricsBatchDensityEnum_unknownDefaultOpenApi,
  ],
);

const ClientMetricsBatchThemeEnum _$clientMetricsBatchThemeEnum_light =
    const ClientMetricsBatchThemeEnum._('light');
const ClientMetricsBatchThemeEnum _$clientMetricsBatchThemeEnum_dark =
    const ClientMetricsBatchThemeEnum._('dark');
const ClientMetricsBatchThemeEnum
_$clientMetricsBatchThemeEnum_unknownDefaultOpenApi =
    const ClientMetricsBatchThemeEnum._('unknownDefaultOpenApi');

ClientMetricsBatchThemeEnum _$clientMetricsBatchThemeEnumValueOf(String name) {
  switch (name) {
    case 'light':
      return _$clientMetricsBatchThemeEnum_light;
    case 'dark':
      return _$clientMetricsBatchThemeEnum_dark;
    case 'unknownDefaultOpenApi':
      return _$clientMetricsBatchThemeEnum_unknownDefaultOpenApi;
    default:
      return _$clientMetricsBatchThemeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ClientMetricsBatchThemeEnum>
_$clientMetricsBatchThemeEnumValues = BuiltSet<ClientMetricsBatchThemeEnum>(
  const <ClientMetricsBatchThemeEnum>[
    _$clientMetricsBatchThemeEnum_light,
    _$clientMetricsBatchThemeEnum_dark,
    _$clientMetricsBatchThemeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<ClientMetricsBatchPlatformEnum>
_$clientMetricsBatchPlatformEnumSerializer =
    _$ClientMetricsBatchPlatformEnumSerializer();
Serializer<ClientMetricsBatchDensityEnum>
_$clientMetricsBatchDensityEnumSerializer =
    _$ClientMetricsBatchDensityEnumSerializer();
Serializer<ClientMetricsBatchThemeEnum>
_$clientMetricsBatchThemeEnumSerializer =
    _$ClientMetricsBatchThemeEnumSerializer();

class _$ClientMetricsBatchPlatformEnumSerializer
    implements PrimitiveSerializer<ClientMetricsBatchPlatformEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'ios': 'ios',
    'android': 'android',
    'macos': 'macos',
    'windows': 'windows',
    'web': 'web',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'ios': 'ios',
    'android': 'android',
    'macos': 'macos',
    'windows': 'windows',
    'web': 'web',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ClientMetricsBatchPlatformEnum];
  @override
  final String wireName = 'ClientMetricsBatchPlatformEnum';

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricsBatchPlatformEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ClientMetricsBatchPlatformEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ClientMetricsBatchPlatformEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ClientMetricsBatchDensityEnumSerializer
    implements PrimitiveSerializer<ClientMetricsBatchDensityEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'comfortable': 'comfortable',
    'compact': 'compact',
    'touch': 'touch',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'comfortable': 'comfortable',
    'compact': 'compact',
    'touch': 'touch',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ClientMetricsBatchDensityEnum];
  @override
  final String wireName = 'ClientMetricsBatchDensityEnum';

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricsBatchDensityEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ClientMetricsBatchDensityEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ClientMetricsBatchDensityEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ClientMetricsBatchThemeEnumSerializer
    implements PrimitiveSerializer<ClientMetricsBatchThemeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'light': 'light',
    'dark': 'dark',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'light': 'light',
    'dark': 'dark',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ClientMetricsBatchThemeEnum];
  @override
  final String wireName = 'ClientMetricsBatchThemeEnum';

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricsBatchThemeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ClientMetricsBatchThemeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ClientMetricsBatchThemeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ClientMetricsBatch extends ClientMetricsBatch {
  @override
  final ClientMetricsBatchPlatformEnum platform;
  @override
  final String appVersion;
  @override
  final ClientMetricsBatchDensityEnum? density;
  @override
  final ClientMetricsBatchThemeEnum? theme;
  @override
  final BuiltList<ClientMetric> metrics;

  factory _$ClientMetricsBatch([
    void Function(ClientMetricsBatchBuilder)? updates,
  ]) => (ClientMetricsBatchBuilder()..update(updates))._build();

  _$ClientMetricsBatch._({
    required this.platform,
    required this.appVersion,
    this.density,
    this.theme,
    required this.metrics,
  }) : super._();
  @override
  ClientMetricsBatch rebuild(
    void Function(ClientMetricsBatchBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ClientMetricsBatchBuilder toBuilder() =>
      ClientMetricsBatchBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ClientMetricsBatch &&
        platform == other.platform &&
        appVersion == other.appVersion &&
        density == other.density &&
        theme == other.theme &&
        metrics == other.metrics;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, appVersion.hashCode);
    _$hash = $jc(_$hash, density.hashCode);
    _$hash = $jc(_$hash, theme.hashCode);
    _$hash = $jc(_$hash, metrics.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ClientMetricsBatch')
          ..add('platform', platform)
          ..add('appVersion', appVersion)
          ..add('density', density)
          ..add('theme', theme)
          ..add('metrics', metrics))
        .toString();
  }
}

class ClientMetricsBatchBuilder
    implements Builder<ClientMetricsBatch, ClientMetricsBatchBuilder> {
  _$ClientMetricsBatch? _$v;

  ClientMetricsBatchPlatformEnum? _platform;
  ClientMetricsBatchPlatformEnum? get platform => _$this._platform;
  set platform(ClientMetricsBatchPlatformEnum? platform) =>
      _$this._platform = platform;

  String? _appVersion;
  String? get appVersion => _$this._appVersion;
  set appVersion(String? appVersion) => _$this._appVersion = appVersion;

  ClientMetricsBatchDensityEnum? _density;
  ClientMetricsBatchDensityEnum? get density => _$this._density;
  set density(ClientMetricsBatchDensityEnum? density) =>
      _$this._density = density;

  ClientMetricsBatchThemeEnum? _theme;
  ClientMetricsBatchThemeEnum? get theme => _$this._theme;
  set theme(ClientMetricsBatchThemeEnum? theme) => _$this._theme = theme;

  ListBuilder<ClientMetric>? _metrics;
  ListBuilder<ClientMetric> get metrics =>
      _$this._metrics ??= ListBuilder<ClientMetric>();
  set metrics(ListBuilder<ClientMetric>? metrics) => _$this._metrics = metrics;

  ClientMetricsBatchBuilder() {
    ClientMetricsBatch._defaults(this);
  }

  ClientMetricsBatchBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _platform = $v.platform;
      _appVersion = $v.appVersion;
      _density = $v.density;
      _theme = $v.theme;
      _metrics = $v.metrics.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ClientMetricsBatch other) {
    _$v = other as _$ClientMetricsBatch;
  }

  @override
  void update(void Function(ClientMetricsBatchBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ClientMetricsBatch build() => _build();

  _$ClientMetricsBatch _build() {
    _$ClientMetricsBatch _$result;
    try {
      _$result =
          _$v ??
          _$ClientMetricsBatch._(
            platform: BuiltValueNullFieldError.checkNotNull(
              platform,
              r'ClientMetricsBatch',
              'platform',
            ),
            appVersion: BuiltValueNullFieldError.checkNotNull(
              appVersion,
              r'ClientMetricsBatch',
              'appVersion',
            ),
            density: density,
            theme: theme,
            metrics: metrics.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'metrics';
        metrics.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ClientMetricsBatch',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
