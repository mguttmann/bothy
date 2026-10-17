// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'local_credential_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$LocalCredentialResult extends LocalCredentialResult {
  @override
  final bool passwordSet;
  @override
  final bool totpEnabled;
  @override
  final String? totpSecretUri;

  factory _$LocalCredentialResult([
    void Function(LocalCredentialResultBuilder)? updates,
  ]) => (LocalCredentialResultBuilder()..update(updates))._build();

  _$LocalCredentialResult._({
    required this.passwordSet,
    required this.totpEnabled,
    this.totpSecretUri,
  }) : super._();
  @override
  LocalCredentialResult rebuild(
    void Function(LocalCredentialResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  LocalCredentialResultBuilder toBuilder() =>
      LocalCredentialResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is LocalCredentialResult &&
        passwordSet == other.passwordSet &&
        totpEnabled == other.totpEnabled &&
        totpSecretUri == other.totpSecretUri;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, passwordSet.hashCode);
    _$hash = $jc(_$hash, totpEnabled.hashCode);
    _$hash = $jc(_$hash, totpSecretUri.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'LocalCredentialResult')
          ..add('passwordSet', passwordSet)
          ..add('totpEnabled', totpEnabled)
          ..add('totpSecretUri', totpSecretUri))
        .toString();
  }
}

class LocalCredentialResultBuilder
    implements Builder<LocalCredentialResult, LocalCredentialResultBuilder> {
  _$LocalCredentialResult? _$v;

  bool? _passwordSet;
  bool? get passwordSet => _$this._passwordSet;
  set passwordSet(bool? passwordSet) => _$this._passwordSet = passwordSet;

  bool? _totpEnabled;
  bool? get totpEnabled => _$this._totpEnabled;
  set totpEnabled(bool? totpEnabled) => _$this._totpEnabled = totpEnabled;

  String? _totpSecretUri;
  String? get totpSecretUri => _$this._totpSecretUri;
  set totpSecretUri(String? totpSecretUri) =>
      _$this._totpSecretUri = totpSecretUri;

  LocalCredentialResultBuilder() {
    LocalCredentialResult._defaults(this);
  }

  LocalCredentialResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _passwordSet = $v.passwordSet;
      _totpEnabled = $v.totpEnabled;
      _totpSecretUri = $v.totpSecretUri;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(LocalCredentialResult other) {
    _$v = other as _$LocalCredentialResult;
  }

  @override
  void update(void Function(LocalCredentialResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  LocalCredentialResult build() => _build();

  _$LocalCredentialResult _build() {
    final _$result =
        _$v ??
        _$LocalCredentialResult._(
          passwordSet: BuiltValueNullFieldError.checkNotNull(
            passwordSet,
            r'LocalCredentialResult',
            'passwordSet',
          ),
          totpEnabled: BuiltValueNullFieldError.checkNotNull(
            totpEnabled,
            r'LocalCredentialResult',
            'totpEnabled',
          ),
          totpSecretUri: totpSecretUri,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
