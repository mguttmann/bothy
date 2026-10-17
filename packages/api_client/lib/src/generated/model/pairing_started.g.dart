// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pairing_started.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PairingStarted extends PairingStarted {
  @override
  final String pairingId;
  @override
  final String pairingCode;
  @override
  final DateTime expiresAt;

  factory _$PairingStarted([void Function(PairingStartedBuilder)? updates]) =>
      (PairingStartedBuilder()..update(updates))._build();

  _$PairingStarted._({
    required this.pairingId,
    required this.pairingCode,
    required this.expiresAt,
  }) : super._();
  @override
  PairingStarted rebuild(void Function(PairingStartedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PairingStartedBuilder toBuilder() => PairingStartedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PairingStarted &&
        pairingId == other.pairingId &&
        pairingCode == other.pairingCode &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pairingId.hashCode);
    _$hash = $jc(_$hash, pairingCode.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PairingStarted')
          ..add('pairingId', pairingId)
          ..add('pairingCode', pairingCode)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class PairingStartedBuilder
    implements Builder<PairingStarted, PairingStartedBuilder> {
  _$PairingStarted? _$v;

  String? _pairingId;
  String? get pairingId => _$this._pairingId;
  set pairingId(String? pairingId) => _$this._pairingId = pairingId;

  String? _pairingCode;
  String? get pairingCode => _$this._pairingCode;
  set pairingCode(String? pairingCode) => _$this._pairingCode = pairingCode;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  PairingStartedBuilder() {
    PairingStarted._defaults(this);
  }

  PairingStartedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pairingId = $v.pairingId;
      _pairingCode = $v.pairingCode;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PairingStarted other) {
    _$v = other as _$PairingStarted;
  }

  @override
  void update(void Function(PairingStartedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PairingStarted build() => _build();

  _$PairingStarted _build() {
    final _$result =
        _$v ??
        _$PairingStarted._(
          pairingId: BuiltValueNullFieldError.checkNotNull(
            pairingId,
            r'PairingStarted',
            'pairingId',
          ),
          pairingCode: BuiltValueNullFieldError.checkNotNull(
            pairingCode,
            r'PairingStarted',
            'pairingCode',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'PairingStarted',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
