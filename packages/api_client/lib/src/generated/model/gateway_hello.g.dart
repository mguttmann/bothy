// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_hello.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayHello extends GatewayHello {
  @override
  final int heartbeatIntervalMs;
  @override
  final int resumeWindowMs;
  @override
  final BuiltList<int> protocolVersions;
  @override
  final String dpopNonce;
  @override
  final DateTime serverTime;
  @override
  final String syncEpoch;

  factory _$GatewayHello([void Function(GatewayHelloBuilder)? updates]) =>
      (GatewayHelloBuilder()..update(updates))._build();

  _$GatewayHello._({
    required this.heartbeatIntervalMs,
    required this.resumeWindowMs,
    required this.protocolVersions,
    required this.dpopNonce,
    required this.serverTime,
    required this.syncEpoch,
  }) : super._();
  @override
  GatewayHello rebuild(void Function(GatewayHelloBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayHelloBuilder toBuilder() => GatewayHelloBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayHello &&
        heartbeatIntervalMs == other.heartbeatIntervalMs &&
        resumeWindowMs == other.resumeWindowMs &&
        protocolVersions == other.protocolVersions &&
        dpopNonce == other.dpopNonce &&
        serverTime == other.serverTime &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, heartbeatIntervalMs.hashCode);
    _$hash = $jc(_$hash, resumeWindowMs.hashCode);
    _$hash = $jc(_$hash, protocolVersions.hashCode);
    _$hash = $jc(_$hash, dpopNonce.hashCode);
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayHello')
          ..add('heartbeatIntervalMs', heartbeatIntervalMs)
          ..add('resumeWindowMs', resumeWindowMs)
          ..add('protocolVersions', protocolVersions)
          ..add('dpopNonce', dpopNonce)
          ..add('serverTime', serverTime)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class GatewayHelloBuilder
    implements Builder<GatewayHello, GatewayHelloBuilder> {
  _$GatewayHello? _$v;

  int? _heartbeatIntervalMs;
  int? get heartbeatIntervalMs => _$this._heartbeatIntervalMs;
  set heartbeatIntervalMs(int? heartbeatIntervalMs) =>
      _$this._heartbeatIntervalMs = heartbeatIntervalMs;

  int? _resumeWindowMs;
  int? get resumeWindowMs => _$this._resumeWindowMs;
  set resumeWindowMs(int? resumeWindowMs) =>
      _$this._resumeWindowMs = resumeWindowMs;

  ListBuilder<int>? _protocolVersions;
  ListBuilder<int> get protocolVersions =>
      _$this._protocolVersions ??= ListBuilder<int>();
  set protocolVersions(ListBuilder<int>? protocolVersions) =>
      _$this._protocolVersions = protocolVersions;

  String? _dpopNonce;
  String? get dpopNonce => _$this._dpopNonce;
  set dpopNonce(String? dpopNonce) => _$this._dpopNonce = dpopNonce;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayHelloBuilder() {
    GatewayHello._defaults(this);
  }

  GatewayHelloBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _heartbeatIntervalMs = $v.heartbeatIntervalMs;
      _resumeWindowMs = $v.resumeWindowMs;
      _protocolVersions = $v.protocolVersions.toBuilder();
      _dpopNonce = $v.dpopNonce;
      _serverTime = $v.serverTime;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayHello other) {
    _$v = other as _$GatewayHello;
  }

  @override
  void update(void Function(GatewayHelloBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayHello build() => _build();

  _$GatewayHello _build() {
    _$GatewayHello _$result;
    try {
      _$result =
          _$v ??
          _$GatewayHello._(
            heartbeatIntervalMs: BuiltValueNullFieldError.checkNotNull(
              heartbeatIntervalMs,
              r'GatewayHello',
              'heartbeatIntervalMs',
            ),
            resumeWindowMs: BuiltValueNullFieldError.checkNotNull(
              resumeWindowMs,
              r'GatewayHello',
              'resumeWindowMs',
            ),
            protocolVersions: protocolVersions.build(),
            dpopNonce: BuiltValueNullFieldError.checkNotNull(
              dpopNonce,
              r'GatewayHello',
              'dpopNonce',
            ),
            serverTime: BuiltValueNullFieldError.checkNotNull(
              serverTime,
              r'GatewayHello',
              'serverTime',
            ),
            syncEpoch: BuiltValueNullFieldError.checkNotNull(
              syncEpoch,
              r'GatewayHello',
              'syncEpoch',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'protocolVersions';
        protocolVersions.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayHello',
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
