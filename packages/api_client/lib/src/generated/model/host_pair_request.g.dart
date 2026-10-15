// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_pair_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostPairRequest extends HostPairRequest {
  @override
  final String pairingCode;
  @override
  final String noisePublicKey;
  @override
  final BuiltMap<String, JsonObject?> authPublicJwk;
  @override
  final HostPlatform platform;
  @override
  final String? osVersion;
  @override
  final String hostVersion;
  @override
  final BuiltList<HostProfile> profiles;
  @override
  final BuiltList<HostReportedSessionUser> sessionUsers;

  factory _$HostPairRequest([void Function(HostPairRequestBuilder)? updates]) =>
      (HostPairRequestBuilder()..update(updates))._build();

  _$HostPairRequest._({
    required this.pairingCode,
    required this.noisePublicKey,
    required this.authPublicJwk,
    required this.platform,
    this.osVersion,
    required this.hostVersion,
    required this.profiles,
    required this.sessionUsers,
  }) : super._();
  @override
  HostPairRequest rebuild(void Function(HostPairRequestBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostPairRequestBuilder toBuilder() => HostPairRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostPairRequest &&
        pairingCode == other.pairingCode &&
        noisePublicKey == other.noisePublicKey &&
        authPublicJwk == other.authPublicJwk &&
        platform == other.platform &&
        osVersion == other.osVersion &&
        hostVersion == other.hostVersion &&
        profiles == other.profiles &&
        sessionUsers == other.sessionUsers;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pairingCode.hashCode);
    _$hash = $jc(_$hash, noisePublicKey.hashCode);
    _$hash = $jc(_$hash, authPublicJwk.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, osVersion.hashCode);
    _$hash = $jc(_$hash, hostVersion.hashCode);
    _$hash = $jc(_$hash, profiles.hashCode);
    _$hash = $jc(_$hash, sessionUsers.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostPairRequest')
          ..add('pairingCode', pairingCode)
          ..add('noisePublicKey', noisePublicKey)
          ..add('authPublicJwk', authPublicJwk)
          ..add('platform', platform)
          ..add('osVersion', osVersion)
          ..add('hostVersion', hostVersion)
          ..add('profiles', profiles)
          ..add('sessionUsers', sessionUsers))
        .toString();
  }
}

class HostPairRequestBuilder
    implements Builder<HostPairRequest, HostPairRequestBuilder> {
  _$HostPairRequest? _$v;

  String? _pairingCode;
  String? get pairingCode => _$this._pairingCode;
  set pairingCode(String? pairingCode) => _$this._pairingCode = pairingCode;

  String? _noisePublicKey;
  String? get noisePublicKey => _$this._noisePublicKey;
  set noisePublicKey(String? noisePublicKey) =>
      _$this._noisePublicKey = noisePublicKey;

  MapBuilder<String, JsonObject?>? _authPublicJwk;
  MapBuilder<String, JsonObject?> get authPublicJwk =>
      _$this._authPublicJwk ??= MapBuilder<String, JsonObject?>();
  set authPublicJwk(MapBuilder<String, JsonObject?>? authPublicJwk) =>
      _$this._authPublicJwk = authPublicJwk;

  HostPlatform? _platform;
  HostPlatform? get platform => _$this._platform;
  set platform(HostPlatform? platform) => _$this._platform = platform;

  String? _osVersion;
  String? get osVersion => _$this._osVersion;
  set osVersion(String? osVersion) => _$this._osVersion = osVersion;

  String? _hostVersion;
  String? get hostVersion => _$this._hostVersion;
  set hostVersion(String? hostVersion) => _$this._hostVersion = hostVersion;

  ListBuilder<HostProfile>? _profiles;
  ListBuilder<HostProfile> get profiles =>
      _$this._profiles ??= ListBuilder<HostProfile>();
  set profiles(ListBuilder<HostProfile>? profiles) =>
      _$this._profiles = profiles;

  ListBuilder<HostReportedSessionUser>? _sessionUsers;
  ListBuilder<HostReportedSessionUser> get sessionUsers =>
      _$this._sessionUsers ??= ListBuilder<HostReportedSessionUser>();
  set sessionUsers(ListBuilder<HostReportedSessionUser>? sessionUsers) =>
      _$this._sessionUsers = sessionUsers;

  HostPairRequestBuilder() {
    HostPairRequest._defaults(this);
  }

  HostPairRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pairingCode = $v.pairingCode;
      _noisePublicKey = $v.noisePublicKey;
      _authPublicJwk = $v.authPublicJwk.toBuilder();
      _platform = $v.platform;
      _osVersion = $v.osVersion;
      _hostVersion = $v.hostVersion;
      _profiles = $v.profiles.toBuilder();
      _sessionUsers = $v.sessionUsers.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostPairRequest other) {
    _$v = other as _$HostPairRequest;
  }

  @override
  void update(void Function(HostPairRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostPairRequest build() => _build();

  _$HostPairRequest _build() {
    _$HostPairRequest _$result;
    try {
      _$result =
          _$v ??
          _$HostPairRequest._(
            pairingCode: BuiltValueNullFieldError.checkNotNull(
              pairingCode,
              r'HostPairRequest',
              'pairingCode',
            ),
            noisePublicKey: BuiltValueNullFieldError.checkNotNull(
              noisePublicKey,
              r'HostPairRequest',
              'noisePublicKey',
            ),
            authPublicJwk: authPublicJwk.build(),
            platform: BuiltValueNullFieldError.checkNotNull(
              platform,
              r'HostPairRequest',
              'platform',
            ),
            osVersion: osVersion,
            hostVersion: BuiltValueNullFieldError.checkNotNull(
              hostVersion,
              r'HostPairRequest',
              'hostVersion',
            ),
            profiles: profiles.build(),
            sessionUsers: sessionUsers.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'authPublicJwk';
        authPublicJwk.build();

        _$failedField = 'profiles';
        profiles.build();
        _$failedField = 'sessionUsers';
        sessionUsers.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostPairRequest',
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
