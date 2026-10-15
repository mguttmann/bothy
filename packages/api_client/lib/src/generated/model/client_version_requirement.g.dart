// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'client_version_requirement.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ClientVersionRequirement extends ClientVersionRequirement {
  @override
  final DevicePlatform platform;
  @override
  final String minVersion;
  @override
  final String? recommendedVersion;

  factory _$ClientVersionRequirement([
    void Function(ClientVersionRequirementBuilder)? updates,
  ]) => (ClientVersionRequirementBuilder()..update(updates))._build();

  _$ClientVersionRequirement._({
    required this.platform,
    required this.minVersion,
    this.recommendedVersion,
  }) : super._();
  @override
  ClientVersionRequirement rebuild(
    void Function(ClientVersionRequirementBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ClientVersionRequirementBuilder toBuilder() =>
      ClientVersionRequirementBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ClientVersionRequirement &&
        platform == other.platform &&
        minVersion == other.minVersion &&
        recommendedVersion == other.recommendedVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, minVersion.hashCode);
    _$hash = $jc(_$hash, recommendedVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ClientVersionRequirement')
          ..add('platform', platform)
          ..add('minVersion', minVersion)
          ..add('recommendedVersion', recommendedVersion))
        .toString();
  }
}

class ClientVersionRequirementBuilder
    implements
        Builder<ClientVersionRequirement, ClientVersionRequirementBuilder> {
  _$ClientVersionRequirement? _$v;

  DevicePlatform? _platform;
  DevicePlatform? get platform => _$this._platform;
  set platform(DevicePlatform? platform) => _$this._platform = platform;

  String? _minVersion;
  String? get minVersion => _$this._minVersion;
  set minVersion(String? minVersion) => _$this._minVersion = minVersion;

  String? _recommendedVersion;
  String? get recommendedVersion => _$this._recommendedVersion;
  set recommendedVersion(String? recommendedVersion) =>
      _$this._recommendedVersion = recommendedVersion;

  ClientVersionRequirementBuilder() {
    ClientVersionRequirement._defaults(this);
  }

  ClientVersionRequirementBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _platform = $v.platform;
      _minVersion = $v.minVersion;
      _recommendedVersion = $v.recommendedVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ClientVersionRequirement other) {
    _$v = other as _$ClientVersionRequirement;
  }

  @override
  void update(void Function(ClientVersionRequirementBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ClientVersionRequirement build() => _build();

  _$ClientVersionRequirement _build() {
    final _$result =
        _$v ??
        _$ClientVersionRequirement._(
          platform: BuiltValueNullFieldError.checkNotNull(
            platform,
            r'ClientVersionRequirement',
            'platform',
          ),
          minVersion: BuiltValueNullFieldError.checkNotNull(
            minVersion,
            r'ClientVersionRequirement',
            'minVersion',
          ),
          recommendedVersion: recommendedVersion,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
