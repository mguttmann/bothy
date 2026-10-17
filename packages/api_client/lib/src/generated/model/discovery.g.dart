// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'discovery.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Discovery extends Discovery {
  @override
  final String instanceId;
  @override
  final bool setupRequired;
  @override
  final BuiltList<String> apiVersions;
  @override
  final String specVersion;
  @override
  final BuiltList<String>? features;
  @override
  final GatewayInfo gateway;
  @override
  final PublicBranding branding;
  @override
  final BuiltList<AuthProviderInfo> authProviders;
  @override
  final bool? localPasswordEnabled;
  @override
  final String jwksUri;
  @override
  final String webhookKeysUri;
  @override
  final BuiltList<ClientVersionRequirement>? minClientVersions;

  factory _$Discovery([void Function(DiscoveryBuilder)? updates]) =>
      (DiscoveryBuilder()..update(updates))._build();

  _$Discovery._({
    required this.instanceId,
    required this.setupRequired,
    required this.apiVersions,
    required this.specVersion,
    this.features,
    required this.gateway,
    required this.branding,
    required this.authProviders,
    this.localPasswordEnabled,
    required this.jwksUri,
    required this.webhookKeysUri,
    this.minClientVersions,
  }) : super._();
  @override
  Discovery rebuild(void Function(DiscoveryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DiscoveryBuilder toBuilder() => DiscoveryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Discovery &&
        instanceId == other.instanceId &&
        setupRequired == other.setupRequired &&
        apiVersions == other.apiVersions &&
        specVersion == other.specVersion &&
        features == other.features &&
        gateway == other.gateway &&
        branding == other.branding &&
        authProviders == other.authProviders &&
        localPasswordEnabled == other.localPasswordEnabled &&
        jwksUri == other.jwksUri &&
        webhookKeysUri == other.webhookKeysUri &&
        minClientVersions == other.minClientVersions;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, instanceId.hashCode);
    _$hash = $jc(_$hash, setupRequired.hashCode);
    _$hash = $jc(_$hash, apiVersions.hashCode);
    _$hash = $jc(_$hash, specVersion.hashCode);
    _$hash = $jc(_$hash, features.hashCode);
    _$hash = $jc(_$hash, gateway.hashCode);
    _$hash = $jc(_$hash, branding.hashCode);
    _$hash = $jc(_$hash, authProviders.hashCode);
    _$hash = $jc(_$hash, localPasswordEnabled.hashCode);
    _$hash = $jc(_$hash, jwksUri.hashCode);
    _$hash = $jc(_$hash, webhookKeysUri.hashCode);
    _$hash = $jc(_$hash, minClientVersions.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Discovery')
          ..add('instanceId', instanceId)
          ..add('setupRequired', setupRequired)
          ..add('apiVersions', apiVersions)
          ..add('specVersion', specVersion)
          ..add('features', features)
          ..add('gateway', gateway)
          ..add('branding', branding)
          ..add('authProviders', authProviders)
          ..add('localPasswordEnabled', localPasswordEnabled)
          ..add('jwksUri', jwksUri)
          ..add('webhookKeysUri', webhookKeysUri)
          ..add('minClientVersions', minClientVersions))
        .toString();
  }
}

class DiscoveryBuilder implements Builder<Discovery, DiscoveryBuilder> {
  _$Discovery? _$v;

  String? _instanceId;
  String? get instanceId => _$this._instanceId;
  set instanceId(String? instanceId) => _$this._instanceId = instanceId;

  bool? _setupRequired;
  bool? get setupRequired => _$this._setupRequired;
  set setupRequired(bool? setupRequired) =>
      _$this._setupRequired = setupRequired;

  ListBuilder<String>? _apiVersions;
  ListBuilder<String> get apiVersions =>
      _$this._apiVersions ??= ListBuilder<String>();
  set apiVersions(ListBuilder<String>? apiVersions) =>
      _$this._apiVersions = apiVersions;

  String? _specVersion;
  String? get specVersion => _$this._specVersion;
  set specVersion(String? specVersion) => _$this._specVersion = specVersion;

  ListBuilder<String>? _features;
  ListBuilder<String> get features =>
      _$this._features ??= ListBuilder<String>();
  set features(ListBuilder<String>? features) => _$this._features = features;

  GatewayInfoBuilder? _gateway;
  GatewayInfoBuilder get gateway => _$this._gateway ??= GatewayInfoBuilder();
  set gateway(GatewayInfoBuilder? gateway) => _$this._gateway = gateway;

  PublicBrandingBuilder? _branding;
  PublicBrandingBuilder get branding =>
      _$this._branding ??= PublicBrandingBuilder();
  set branding(PublicBrandingBuilder? branding) => _$this._branding = branding;

  ListBuilder<AuthProviderInfo>? _authProviders;
  ListBuilder<AuthProviderInfo> get authProviders =>
      _$this._authProviders ??= ListBuilder<AuthProviderInfo>();
  set authProviders(ListBuilder<AuthProviderInfo>? authProviders) =>
      _$this._authProviders = authProviders;

  bool? _localPasswordEnabled;
  bool? get localPasswordEnabled => _$this._localPasswordEnabled;
  set localPasswordEnabled(bool? localPasswordEnabled) =>
      _$this._localPasswordEnabled = localPasswordEnabled;

  String? _jwksUri;
  String? get jwksUri => _$this._jwksUri;
  set jwksUri(String? jwksUri) => _$this._jwksUri = jwksUri;

  String? _webhookKeysUri;
  String? get webhookKeysUri => _$this._webhookKeysUri;
  set webhookKeysUri(String? webhookKeysUri) =>
      _$this._webhookKeysUri = webhookKeysUri;

  ListBuilder<ClientVersionRequirement>? _minClientVersions;
  ListBuilder<ClientVersionRequirement> get minClientVersions =>
      _$this._minClientVersions ??= ListBuilder<ClientVersionRequirement>();
  set minClientVersions(
    ListBuilder<ClientVersionRequirement>? minClientVersions,
  ) => _$this._minClientVersions = minClientVersions;

  DiscoveryBuilder() {
    Discovery._defaults(this);
  }

  DiscoveryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _instanceId = $v.instanceId;
      _setupRequired = $v.setupRequired;
      _apiVersions = $v.apiVersions.toBuilder();
      _specVersion = $v.specVersion;
      _features = $v.features?.toBuilder();
      _gateway = $v.gateway.toBuilder();
      _branding = $v.branding.toBuilder();
      _authProviders = $v.authProviders.toBuilder();
      _localPasswordEnabled = $v.localPasswordEnabled;
      _jwksUri = $v.jwksUri;
      _webhookKeysUri = $v.webhookKeysUri;
      _minClientVersions = $v.minClientVersions?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Discovery other) {
    _$v = other as _$Discovery;
  }

  @override
  void update(void Function(DiscoveryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Discovery build() => _build();

  _$Discovery _build() {
    _$Discovery _$result;
    try {
      _$result =
          _$v ??
          _$Discovery._(
            instanceId: BuiltValueNullFieldError.checkNotNull(
              instanceId,
              r'Discovery',
              'instanceId',
            ),
            setupRequired: BuiltValueNullFieldError.checkNotNull(
              setupRequired,
              r'Discovery',
              'setupRequired',
            ),
            apiVersions: apiVersions.build(),
            specVersion: BuiltValueNullFieldError.checkNotNull(
              specVersion,
              r'Discovery',
              'specVersion',
            ),
            features: _features?.build(),
            gateway: gateway.build(),
            branding: branding.build(),
            authProviders: authProviders.build(),
            localPasswordEnabled: localPasswordEnabled,
            jwksUri: BuiltValueNullFieldError.checkNotNull(
              jwksUri,
              r'Discovery',
              'jwksUri',
            ),
            webhookKeysUri: BuiltValueNullFieldError.checkNotNull(
              webhookKeysUri,
              r'Discovery',
              'webhookKeysUri',
            ),
            minClientVersions: _minClientVersions?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'apiVersions';
        apiVersions.build();

        _$failedField = 'features';
        _features?.build();
        _$failedField = 'gateway';
        gateway.build();
        _$failedField = 'branding';
        branding.build();
        _$failedField = 'authProviders';
        authProviders.build();

        _$failedField = 'minClientVersions';
        _minClientVersions?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Discovery',
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
