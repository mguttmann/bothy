// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'credential_holder_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CredentialHolderSet extends CredentialHolderSet {
  @override
  final String credentialHolderId;
  @override
  final String reason;

  factory _$CredentialHolderSet([
    void Function(CredentialHolderSetBuilder)? updates,
  ]) => (CredentialHolderSetBuilder()..update(updates))._build();

  _$CredentialHolderSet._({
    required this.credentialHolderId,
    required this.reason,
  }) : super._();
  @override
  CredentialHolderSet rebuild(
    void Function(CredentialHolderSetBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CredentialHolderSetBuilder toBuilder() =>
      CredentialHolderSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CredentialHolderSet &&
        credentialHolderId == other.credentialHolderId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CredentialHolderSet')
          ..add('credentialHolderId', credentialHolderId)
          ..add('reason', reason))
        .toString();
  }
}

class CredentialHolderSetBuilder
    implements Builder<CredentialHolderSet, CredentialHolderSetBuilder> {
  _$CredentialHolderSet? _$v;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  CredentialHolderSetBuilder() {
    CredentialHolderSet._defaults(this);
  }

  CredentialHolderSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _credentialHolderId = $v.credentialHolderId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CredentialHolderSet other) {
    _$v = other as _$CredentialHolderSet;
  }

  @override
  void update(void Function(CredentialHolderSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CredentialHolderSet build() => _build();

  _$CredentialHolderSet _build() {
    final _$result =
        _$v ??
        _$CredentialHolderSet._(
          credentialHolderId: BuiltValueNullFieldError.checkNotNull(
            credentialHolderId,
            r'CredentialHolderSet',
            'credentialHolderId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'CredentialHolderSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
