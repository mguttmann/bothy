// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_client_info.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayClientInfo extends GatewayClientInfo {
  @override
  final DevicePlatform platform;
  @override
  final String version;

  factory _$GatewayClientInfo([
    void Function(GatewayClientInfoBuilder)? updates,
  ]) => (GatewayClientInfoBuilder()..update(updates))._build();

  _$GatewayClientInfo._({required this.platform, required this.version})
    : super._();
  @override
  GatewayClientInfo rebuild(void Function(GatewayClientInfoBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayClientInfoBuilder toBuilder() =>
      GatewayClientInfoBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayClientInfo &&
        platform == other.platform &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayClientInfo')
          ..add('platform', platform)
          ..add('version', version))
        .toString();
  }
}

class GatewayClientInfoBuilder
    implements Builder<GatewayClientInfo, GatewayClientInfoBuilder> {
  _$GatewayClientInfo? _$v;

  DevicePlatform? _platform;
  DevicePlatform? get platform => _$this._platform;
  set platform(DevicePlatform? platform) => _$this._platform = platform;

  String? _version;
  String? get version => _$this._version;
  set version(String? version) => _$this._version = version;

  GatewayClientInfoBuilder() {
    GatewayClientInfo._defaults(this);
  }

  GatewayClientInfoBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _platform = $v.platform;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayClientInfo other) {
    _$v = other as _$GatewayClientInfo;
  }

  @override
  void update(void Function(GatewayClientInfoBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayClientInfo build() => _build();

  _$GatewayClientInfo _build() {
    final _$result =
        _$v ??
        _$GatewayClientInfo._(
          platform: BuiltValueNullFieldError.checkNotNull(
            platform,
            r'GatewayClientInfo',
            'platform',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'GatewayClientInfo',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
