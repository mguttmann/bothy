// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_pairing_code.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostPairingCode extends HostPairingCode {
  @override
  final String hostId;
  @override
  final String pairingCode;
  @override
  final DateTime expiresAt;

  factory _$HostPairingCode([void Function(HostPairingCodeBuilder)? updates]) =>
      (HostPairingCodeBuilder()..update(updates))._build();

  _$HostPairingCode._({
    required this.hostId,
    required this.pairingCode,
    required this.expiresAt,
  }) : super._();
  @override
  HostPairingCode rebuild(void Function(HostPairingCodeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostPairingCodeBuilder toBuilder() => HostPairingCodeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostPairingCode &&
        hostId == other.hostId &&
        pairingCode == other.pairingCode &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, pairingCode.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostPairingCode')
          ..add('hostId', hostId)
          ..add('pairingCode', pairingCode)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class HostPairingCodeBuilder
    implements Builder<HostPairingCode, HostPairingCodeBuilder> {
  _$HostPairingCode? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _pairingCode;
  String? get pairingCode => _$this._pairingCode;
  set pairingCode(String? pairingCode) => _$this._pairingCode = pairingCode;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  HostPairingCodeBuilder() {
    HostPairingCode._defaults(this);
  }

  HostPairingCodeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _pairingCode = $v.pairingCode;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostPairingCode other) {
    _$v = other as _$HostPairingCode;
  }

  @override
  void update(void Function(HostPairingCodeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostPairingCode build() => _build();

  _$HostPairingCode _build() {
    final _$result =
        _$v ??
        _$HostPairingCode._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'HostPairingCode',
            'hostId',
          ),
          pairingCode: BuiltValueNullFieldError.checkNotNull(
            pairingCode,
            r'HostPairingCode',
            'pairingCode',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'HostPairingCode',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
