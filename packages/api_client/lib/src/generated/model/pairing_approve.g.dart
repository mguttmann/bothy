// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pairing_approve.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PairingApprove extends PairingApprove {
  @override
  final String pairingCode;

  factory _$PairingApprove([void Function(PairingApproveBuilder)? updates]) =>
      (PairingApproveBuilder()..update(updates))._build();

  _$PairingApprove._({required this.pairingCode}) : super._();
  @override
  PairingApprove rebuild(void Function(PairingApproveBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PairingApproveBuilder toBuilder() => PairingApproveBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PairingApprove && pairingCode == other.pairingCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pairingCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PairingApprove',
    )..add('pairingCode', pairingCode)).toString();
  }
}

class PairingApproveBuilder
    implements Builder<PairingApprove, PairingApproveBuilder> {
  _$PairingApprove? _$v;

  String? _pairingCode;
  String? get pairingCode => _$this._pairingCode;
  set pairingCode(String? pairingCode) => _$this._pairingCode = pairingCode;

  PairingApproveBuilder() {
    PairingApprove._defaults(this);
  }

  PairingApproveBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pairingCode = $v.pairingCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PairingApprove other) {
    _$v = other as _$PairingApprove;
  }

  @override
  void update(void Function(PairingApproveBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PairingApprove build() => _build();

  _$PairingApprove _build() {
    final _$result =
        _$v ??
        _$PairingApprove._(
          pairingCode: BuiltValueNullFieldError.checkNotNull(
            pairingCode,
            r'PairingApprove',
            'pairingCode',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
