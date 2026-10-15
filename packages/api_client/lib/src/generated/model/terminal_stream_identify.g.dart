// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_stream_identify.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalStreamIdentify extends TerminalStreamIdentify {
  @override
  final int protocolVersion;
  @override
  final String? accessToken;
  @override
  final String? dpop;
  @override
  final String deviceId;

  factory _$TerminalStreamIdentify([
    void Function(TerminalStreamIdentifyBuilder)? updates,
  ]) => (TerminalStreamIdentifyBuilder()..update(updates))._build();

  _$TerminalStreamIdentify._({
    required this.protocolVersion,
    this.accessToken,
    this.dpop,
    required this.deviceId,
  }) : super._();
  @override
  TerminalStreamIdentify rebuild(
    void Function(TerminalStreamIdentifyBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalStreamIdentifyBuilder toBuilder() =>
      TerminalStreamIdentifyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalStreamIdentify &&
        protocolVersion == other.protocolVersion &&
        accessToken == other.accessToken &&
        dpop == other.dpop &&
        deviceId == other.deviceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersion.hashCode);
    _$hash = $jc(_$hash, accessToken.hashCode);
    _$hash = $jc(_$hash, dpop.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalStreamIdentify')
          ..add('protocolVersion', protocolVersion)
          ..add('accessToken', accessToken)
          ..add('dpop', dpop)
          ..add('deviceId', deviceId))
        .toString();
  }
}

class TerminalStreamIdentifyBuilder
    implements Builder<TerminalStreamIdentify, TerminalStreamIdentifyBuilder> {
  _$TerminalStreamIdentify? _$v;

  int? _protocolVersion;
  int? get protocolVersion => _$this._protocolVersion;
  set protocolVersion(int? protocolVersion) =>
      _$this._protocolVersion = protocolVersion;

  String? _accessToken;
  String? get accessToken => _$this._accessToken;
  set accessToken(String? accessToken) => _$this._accessToken = accessToken;

  String? _dpop;
  String? get dpop => _$this._dpop;
  set dpop(String? dpop) => _$this._dpop = dpop;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  TerminalStreamIdentifyBuilder() {
    TerminalStreamIdentify._defaults(this);
  }

  TerminalStreamIdentifyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersion = $v.protocolVersion;
      _accessToken = $v.accessToken;
      _dpop = $v.dpop;
      _deviceId = $v.deviceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalStreamIdentify other) {
    _$v = other as _$TerminalStreamIdentify;
  }

  @override
  void update(void Function(TerminalStreamIdentifyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalStreamIdentify build() => _build();

  _$TerminalStreamIdentify _build() {
    final _$result =
        _$v ??
        _$TerminalStreamIdentify._(
          protocolVersion: BuiltValueNullFieldError.checkNotNull(
            protocolVersion,
            r'TerminalStreamIdentify',
            'protocolVersion',
          ),
          accessToken: accessToken,
          dpop: dpop,
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'TerminalStreamIdentify',
            'deviceId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
