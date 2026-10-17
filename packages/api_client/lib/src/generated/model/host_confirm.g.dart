// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_confirm.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostConfirm extends HostConfirm {
  @override
  final String fingerprintPrefix;
  @override
  final LlmCredentials llmCredentials;
  @override
  final String? credentialHolderId;
  @override
  final bool declaredServerHost;
  @override
  final BuiltList<HostSessionUserAssign>? sessionUsers;
  @override
  final String reason;

  factory _$HostConfirm([void Function(HostConfirmBuilder)? updates]) =>
      (HostConfirmBuilder()..update(updates))._build();

  _$HostConfirm._({
    required this.fingerprintPrefix,
    required this.llmCredentials,
    this.credentialHolderId,
    required this.declaredServerHost,
    this.sessionUsers,
    required this.reason,
  }) : super._();
  @override
  HostConfirm rebuild(void Function(HostConfirmBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostConfirmBuilder toBuilder() => HostConfirmBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostConfirm &&
        fingerprintPrefix == other.fingerprintPrefix &&
        llmCredentials == other.llmCredentials &&
        credentialHolderId == other.credentialHolderId &&
        declaredServerHost == other.declaredServerHost &&
        sessionUsers == other.sessionUsers &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fingerprintPrefix.hashCode);
    _$hash = $jc(_$hash, llmCredentials.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, declaredServerHost.hashCode);
    _$hash = $jc(_$hash, sessionUsers.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostConfirm')
          ..add('fingerprintPrefix', fingerprintPrefix)
          ..add('llmCredentials', llmCredentials)
          ..add('credentialHolderId', credentialHolderId)
          ..add('declaredServerHost', declaredServerHost)
          ..add('sessionUsers', sessionUsers)
          ..add('reason', reason))
        .toString();
  }
}

class HostConfirmBuilder implements Builder<HostConfirm, HostConfirmBuilder> {
  _$HostConfirm? _$v;

  String? _fingerprintPrefix;
  String? get fingerprintPrefix => _$this._fingerprintPrefix;
  set fingerprintPrefix(String? fingerprintPrefix) =>
      _$this._fingerprintPrefix = fingerprintPrefix;

  LlmCredentials? _llmCredentials;
  LlmCredentials? get llmCredentials => _$this._llmCredentials;
  set llmCredentials(LlmCredentials? llmCredentials) =>
      _$this._llmCredentials = llmCredentials;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  bool? _declaredServerHost;
  bool? get declaredServerHost => _$this._declaredServerHost;
  set declaredServerHost(bool? declaredServerHost) =>
      _$this._declaredServerHost = declaredServerHost;

  ListBuilder<HostSessionUserAssign>? _sessionUsers;
  ListBuilder<HostSessionUserAssign> get sessionUsers =>
      _$this._sessionUsers ??= ListBuilder<HostSessionUserAssign>();
  set sessionUsers(ListBuilder<HostSessionUserAssign>? sessionUsers) =>
      _$this._sessionUsers = sessionUsers;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostConfirmBuilder() {
    HostConfirm._defaults(this);
  }

  HostConfirmBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fingerprintPrefix = $v.fingerprintPrefix;
      _llmCredentials = $v.llmCredentials;
      _credentialHolderId = $v.credentialHolderId;
      _declaredServerHost = $v.declaredServerHost;
      _sessionUsers = $v.sessionUsers?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostConfirm other) {
    _$v = other as _$HostConfirm;
  }

  @override
  void update(void Function(HostConfirmBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostConfirm build() => _build();

  _$HostConfirm _build() {
    _$HostConfirm _$result;
    try {
      _$result =
          _$v ??
          _$HostConfirm._(
            fingerprintPrefix: BuiltValueNullFieldError.checkNotNull(
              fingerprintPrefix,
              r'HostConfirm',
              'fingerprintPrefix',
            ),
            llmCredentials: BuiltValueNullFieldError.checkNotNull(
              llmCredentials,
              r'HostConfirm',
              'llmCredentials',
            ),
            credentialHolderId: credentialHolderId,
            declaredServerHost: BuiltValueNullFieldError.checkNotNull(
              declaredServerHost,
              r'HostConfirm',
              'declaredServerHost',
            ),
            sessionUsers: _sessionUsers?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'HostConfirm',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'sessionUsers';
        _sessionUsers?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostConfirm',
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
