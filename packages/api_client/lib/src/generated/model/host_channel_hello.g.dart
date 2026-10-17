// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_channel_hello.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostChannelHello extends HostChannelHello {
  @override
  final BuiltList<int> protocolVersions;
  @override
  final String challenge;
  @override
  final DateTime serverTime;
  @override
  final int heartbeatIntervalMs;
  @override
  final Jwks jwks;

  factory _$HostChannelHello([
    void Function(HostChannelHelloBuilder)? updates,
  ]) => (HostChannelHelloBuilder()..update(updates))._build();

  _$HostChannelHello._({
    required this.protocolVersions,
    required this.challenge,
    required this.serverTime,
    required this.heartbeatIntervalMs,
    required this.jwks,
  }) : super._();
  @override
  HostChannelHello rebuild(void Function(HostChannelHelloBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostChannelHelloBuilder toBuilder() =>
      HostChannelHelloBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostChannelHello &&
        protocolVersions == other.protocolVersions &&
        challenge == other.challenge &&
        serverTime == other.serverTime &&
        heartbeatIntervalMs == other.heartbeatIntervalMs &&
        jwks == other.jwks;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersions.hashCode);
    _$hash = $jc(_$hash, challenge.hashCode);
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jc(_$hash, heartbeatIntervalMs.hashCode);
    _$hash = $jc(_$hash, jwks.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostChannelHello')
          ..add('protocolVersions', protocolVersions)
          ..add('challenge', challenge)
          ..add('serverTime', serverTime)
          ..add('heartbeatIntervalMs', heartbeatIntervalMs)
          ..add('jwks', jwks))
        .toString();
  }
}

class HostChannelHelloBuilder
    implements Builder<HostChannelHello, HostChannelHelloBuilder> {
  _$HostChannelHello? _$v;

  ListBuilder<int>? _protocolVersions;
  ListBuilder<int> get protocolVersions =>
      _$this._protocolVersions ??= ListBuilder<int>();
  set protocolVersions(ListBuilder<int>? protocolVersions) =>
      _$this._protocolVersions = protocolVersions;

  String? _challenge;
  String? get challenge => _$this._challenge;
  set challenge(String? challenge) => _$this._challenge = challenge;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  int? _heartbeatIntervalMs;
  int? get heartbeatIntervalMs => _$this._heartbeatIntervalMs;
  set heartbeatIntervalMs(int? heartbeatIntervalMs) =>
      _$this._heartbeatIntervalMs = heartbeatIntervalMs;

  JwksBuilder? _jwks;
  JwksBuilder get jwks => _$this._jwks ??= JwksBuilder();
  set jwks(JwksBuilder? jwks) => _$this._jwks = jwks;

  HostChannelHelloBuilder() {
    HostChannelHello._defaults(this);
  }

  HostChannelHelloBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersions = $v.protocolVersions.toBuilder();
      _challenge = $v.challenge;
      _serverTime = $v.serverTime;
      _heartbeatIntervalMs = $v.heartbeatIntervalMs;
      _jwks = $v.jwks.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostChannelHello other) {
    _$v = other as _$HostChannelHello;
  }

  @override
  void update(void Function(HostChannelHelloBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostChannelHello build() => _build();

  _$HostChannelHello _build() {
    _$HostChannelHello _$result;
    try {
      _$result =
          _$v ??
          _$HostChannelHello._(
            protocolVersions: protocolVersions.build(),
            challenge: BuiltValueNullFieldError.checkNotNull(
              challenge,
              r'HostChannelHello',
              'challenge',
            ),
            serverTime: BuiltValueNullFieldError.checkNotNull(
              serverTime,
              r'HostChannelHello',
              'serverTime',
            ),
            heartbeatIntervalMs: BuiltValueNullFieldError.checkNotNull(
              heartbeatIntervalMs,
              r'HostChannelHello',
              'heartbeatIntervalMs',
            ),
            jwks: jwks.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'protocolVersions';
        protocolVersions.build();

        _$failedField = 'jwks';
        jwks.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostChannelHello',
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
