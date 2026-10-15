// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_llm_credentials_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostLlmCredentialsSet extends HostLlmCredentialsSet {
  @override
  final LlmCredentials llmCredentials;
  @override
  final String? credentialHolderId;
  @override
  final String reason;

  factory _$HostLlmCredentialsSet([
    void Function(HostLlmCredentialsSetBuilder)? updates,
  ]) => (HostLlmCredentialsSetBuilder()..update(updates))._build();

  _$HostLlmCredentialsSet._({
    required this.llmCredentials,
    this.credentialHolderId,
    required this.reason,
  }) : super._();
  @override
  HostLlmCredentialsSet rebuild(
    void Function(HostLlmCredentialsSetBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostLlmCredentialsSetBuilder toBuilder() =>
      HostLlmCredentialsSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostLlmCredentialsSet &&
        llmCredentials == other.llmCredentials &&
        credentialHolderId == other.credentialHolderId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, llmCredentials.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostLlmCredentialsSet')
          ..add('llmCredentials', llmCredentials)
          ..add('credentialHolderId', credentialHolderId)
          ..add('reason', reason))
        .toString();
  }
}

class HostLlmCredentialsSetBuilder
    implements Builder<HostLlmCredentialsSet, HostLlmCredentialsSetBuilder> {
  _$HostLlmCredentialsSet? _$v;

  LlmCredentials? _llmCredentials;
  LlmCredentials? get llmCredentials => _$this._llmCredentials;
  set llmCredentials(LlmCredentials? llmCredentials) =>
      _$this._llmCredentials = llmCredentials;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostLlmCredentialsSetBuilder() {
    HostLlmCredentialsSet._defaults(this);
  }

  HostLlmCredentialsSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _llmCredentials = $v.llmCredentials;
      _credentialHolderId = $v.credentialHolderId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostLlmCredentialsSet other) {
    _$v = other as _$HostLlmCredentialsSet;
  }

  @override
  void update(void Function(HostLlmCredentialsSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostLlmCredentialsSet build() => _build();

  _$HostLlmCredentialsSet _build() {
    final _$result =
        _$v ??
        _$HostLlmCredentialsSet._(
          llmCredentials: BuiltValueNullFieldError.checkNotNull(
            llmCredentials,
            r'HostLlmCredentialsSet',
            'llmCredentials',
          ),
          credentialHolderId: credentialHolderId,
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'HostLlmCredentialsSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
