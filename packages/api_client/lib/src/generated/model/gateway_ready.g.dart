// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_ready.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayReady extends GatewayReady {
  @override
  final String gatewaySessionId;
  @override
  final PrincipalRef principal;
  @override
  final String? deviceId;
  @override
  final int permissionVersion;
  @override
  final int headSeq;
  @override
  final Snapshot snapshot;
  @override
  final String syncEpoch;

  factory _$GatewayReady([void Function(GatewayReadyBuilder)? updates]) =>
      (GatewayReadyBuilder()..update(updates))._build();

  _$GatewayReady._({
    required this.gatewaySessionId,
    required this.principal,
    this.deviceId,
    required this.permissionVersion,
    required this.headSeq,
    required this.snapshot,
    required this.syncEpoch,
  }) : super._();
  @override
  GatewayReady rebuild(void Function(GatewayReadyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayReadyBuilder toBuilder() => GatewayReadyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayReady &&
        gatewaySessionId == other.gatewaySessionId &&
        principal == other.principal &&
        deviceId == other.deviceId &&
        permissionVersion == other.permissionVersion &&
        headSeq == other.headSeq &&
        snapshot == other.snapshot &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, gatewaySessionId.hashCode);
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jc(_$hash, snapshot.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayReady')
          ..add('gatewaySessionId', gatewaySessionId)
          ..add('principal', principal)
          ..add('deviceId', deviceId)
          ..add('permissionVersion', permissionVersion)
          ..add('headSeq', headSeq)
          ..add('snapshot', snapshot)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class GatewayReadyBuilder
    implements Builder<GatewayReady, GatewayReadyBuilder> {
  _$GatewayReady? _$v;

  String? _gatewaySessionId;
  String? get gatewaySessionId => _$this._gatewaySessionId;
  set gatewaySessionId(String? gatewaySessionId) =>
      _$this._gatewaySessionId = gatewaySessionId;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  SnapshotBuilder? _snapshot;
  SnapshotBuilder get snapshot => _$this._snapshot ??= SnapshotBuilder();
  set snapshot(SnapshotBuilder? snapshot) => _$this._snapshot = snapshot;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayReadyBuilder() {
    GatewayReady._defaults(this);
  }

  GatewayReadyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _gatewaySessionId = $v.gatewaySessionId;
      _principal = $v.principal.toBuilder();
      _deviceId = $v.deviceId;
      _permissionVersion = $v.permissionVersion;
      _headSeq = $v.headSeq;
      _snapshot = $v.snapshot.toBuilder();
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayReady other) {
    _$v = other as _$GatewayReady;
  }

  @override
  void update(void Function(GatewayReadyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayReady build() => _build();

  _$GatewayReady _build() {
    _$GatewayReady _$result;
    try {
      _$result =
          _$v ??
          _$GatewayReady._(
            gatewaySessionId: BuiltValueNullFieldError.checkNotNull(
              gatewaySessionId,
              r'GatewayReady',
              'gatewaySessionId',
            ),
            principal: principal.build(),
            deviceId: deviceId,
            permissionVersion: BuiltValueNullFieldError.checkNotNull(
              permissionVersion,
              r'GatewayReady',
              'permissionVersion',
            ),
            headSeq: BuiltValueNullFieldError.checkNotNull(
              headSeq,
              r'GatewayReady',
              'headSeq',
            ),
            snapshot: snapshot.build(),
            syncEpoch: BuiltValueNullFieldError.checkNotNull(
              syncEpoch,
              r'GatewayReady',
              'syncEpoch',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();

        _$failedField = 'snapshot';
        snapshot.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayReady',
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
