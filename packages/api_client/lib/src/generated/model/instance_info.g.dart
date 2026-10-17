// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_info.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceInfo extends InstanceInfo {
  @override
  final String instanceId;
  @override
  final Branding branding;
  @override
  final String localeDefault;
  @override
  final String specVersion;
  @override
  final BuiltList<EvaluatedFlag> features;
  @override
  final InstanceLimits limits;

  factory _$InstanceInfo([void Function(InstanceInfoBuilder)? updates]) =>
      (InstanceInfoBuilder()..update(updates))._build();

  _$InstanceInfo._({
    required this.instanceId,
    required this.branding,
    required this.localeDefault,
    required this.specVersion,
    required this.features,
    required this.limits,
  }) : super._();
  @override
  InstanceInfo rebuild(void Function(InstanceInfoBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  InstanceInfoBuilder toBuilder() => InstanceInfoBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceInfo &&
        instanceId == other.instanceId &&
        branding == other.branding &&
        localeDefault == other.localeDefault &&
        specVersion == other.specVersion &&
        features == other.features &&
        limits == other.limits;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, instanceId.hashCode);
    _$hash = $jc(_$hash, branding.hashCode);
    _$hash = $jc(_$hash, localeDefault.hashCode);
    _$hash = $jc(_$hash, specVersion.hashCode);
    _$hash = $jc(_$hash, features.hashCode);
    _$hash = $jc(_$hash, limits.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InstanceInfo')
          ..add('instanceId', instanceId)
          ..add('branding', branding)
          ..add('localeDefault', localeDefault)
          ..add('specVersion', specVersion)
          ..add('features', features)
          ..add('limits', limits))
        .toString();
  }
}

class InstanceInfoBuilder
    implements Builder<InstanceInfo, InstanceInfoBuilder> {
  _$InstanceInfo? _$v;

  String? _instanceId;
  String? get instanceId => _$this._instanceId;
  set instanceId(String? instanceId) => _$this._instanceId = instanceId;

  BrandingBuilder? _branding;
  BrandingBuilder get branding => _$this._branding ??= BrandingBuilder();
  set branding(BrandingBuilder? branding) => _$this._branding = branding;

  String? _localeDefault;
  String? get localeDefault => _$this._localeDefault;
  set localeDefault(String? localeDefault) =>
      _$this._localeDefault = localeDefault;

  String? _specVersion;
  String? get specVersion => _$this._specVersion;
  set specVersion(String? specVersion) => _$this._specVersion = specVersion;

  ListBuilder<EvaluatedFlag>? _features;
  ListBuilder<EvaluatedFlag> get features =>
      _$this._features ??= ListBuilder<EvaluatedFlag>();
  set features(ListBuilder<EvaluatedFlag>? features) =>
      _$this._features = features;

  InstanceLimitsBuilder? _limits;
  InstanceLimitsBuilder get limits =>
      _$this._limits ??= InstanceLimitsBuilder();
  set limits(InstanceLimitsBuilder? limits) => _$this._limits = limits;

  InstanceInfoBuilder() {
    InstanceInfo._defaults(this);
  }

  InstanceInfoBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _instanceId = $v.instanceId;
      _branding = $v.branding.toBuilder();
      _localeDefault = $v.localeDefault;
      _specVersion = $v.specVersion;
      _features = $v.features.toBuilder();
      _limits = $v.limits.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceInfo other) {
    _$v = other as _$InstanceInfo;
  }

  @override
  void update(void Function(InstanceInfoBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceInfo build() => _build();

  _$InstanceInfo _build() {
    _$InstanceInfo _$result;
    try {
      _$result =
          _$v ??
          _$InstanceInfo._(
            instanceId: BuiltValueNullFieldError.checkNotNull(
              instanceId,
              r'InstanceInfo',
              'instanceId',
            ),
            branding: branding.build(),
            localeDefault: BuiltValueNullFieldError.checkNotNull(
              localeDefault,
              r'InstanceInfo',
              'localeDefault',
            ),
            specVersion: BuiltValueNullFieldError.checkNotNull(
              specVersion,
              r'InstanceInfo',
              'specVersion',
            ),
            features: features.build(),
            limits: limits.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'branding';
        branding.build();

        _$failedField = 'features';
        features.build();
        _$failedField = 'limits';
        limits.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InstanceInfo',
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
