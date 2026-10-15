// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'local_credential_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$LocalCredentialSet extends LocalCredentialSet {
  @override
  final String? password;
  @override
  final bool? totpEnroll;
  @override
  final String? totpCode;

  factory _$LocalCredentialSet([
    void Function(LocalCredentialSetBuilder)? updates,
  ]) => (LocalCredentialSetBuilder()..update(updates))._build();

  _$LocalCredentialSet._({this.password, this.totpEnroll, this.totpCode})
    : super._();
  @override
  LocalCredentialSet rebuild(
    void Function(LocalCredentialSetBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  LocalCredentialSetBuilder toBuilder() =>
      LocalCredentialSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is LocalCredentialSet &&
        password == other.password &&
        totpEnroll == other.totpEnroll &&
        totpCode == other.totpCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, password.hashCode);
    _$hash = $jc(_$hash, totpEnroll.hashCode);
    _$hash = $jc(_$hash, totpCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'LocalCredentialSet')
          ..add('password', password)
          ..add('totpEnroll', totpEnroll)
          ..add('totpCode', totpCode))
        .toString();
  }
}

class LocalCredentialSetBuilder
    implements Builder<LocalCredentialSet, LocalCredentialSetBuilder> {
  _$LocalCredentialSet? _$v;

  String? _password;
  String? get password => _$this._password;
  set password(String? password) => _$this._password = password;

  bool? _totpEnroll;
  bool? get totpEnroll => _$this._totpEnroll;
  set totpEnroll(bool? totpEnroll) => _$this._totpEnroll = totpEnroll;

  String? _totpCode;
  String? get totpCode => _$this._totpCode;
  set totpCode(String? totpCode) => _$this._totpCode = totpCode;

  LocalCredentialSetBuilder() {
    LocalCredentialSet._defaults(this);
  }

  LocalCredentialSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _password = $v.password;
      _totpEnroll = $v.totpEnroll;
      _totpCode = $v.totpCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(LocalCredentialSet other) {
    _$v = other as _$LocalCredentialSet;
  }

  @override
  void update(void Function(LocalCredentialSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  LocalCredentialSet build() => _build();

  _$LocalCredentialSet _build() {
    final _$result =
        _$v ??
        _$LocalCredentialSet._(
          password: password,
          totpEnroll: totpEnroll,
          totpCode: totpCode,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
