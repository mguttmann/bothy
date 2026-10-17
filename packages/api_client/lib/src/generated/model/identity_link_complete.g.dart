// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_link_complete.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityLinkComplete extends IdentityLinkComplete {
  @override
  final String loginCode;
  @override
  final String clientVerifier;

  factory _$IdentityLinkComplete([
    void Function(IdentityLinkCompleteBuilder)? updates,
  ]) => (IdentityLinkCompleteBuilder()..update(updates))._build();

  _$IdentityLinkComplete._({
    required this.loginCode,
    required this.clientVerifier,
  }) : super._();
  @override
  IdentityLinkComplete rebuild(
    void Function(IdentityLinkCompleteBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IdentityLinkCompleteBuilder toBuilder() =>
      IdentityLinkCompleteBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityLinkComplete &&
        loginCode == other.loginCode &&
        clientVerifier == other.clientVerifier;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, loginCode.hashCode);
    _$hash = $jc(_$hash, clientVerifier.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IdentityLinkComplete')
          ..add('loginCode', loginCode)
          ..add('clientVerifier', clientVerifier))
        .toString();
  }
}

class IdentityLinkCompleteBuilder
    implements Builder<IdentityLinkComplete, IdentityLinkCompleteBuilder> {
  _$IdentityLinkComplete? _$v;

  String? _loginCode;
  String? get loginCode => _$this._loginCode;
  set loginCode(String? loginCode) => _$this._loginCode = loginCode;

  String? _clientVerifier;
  String? get clientVerifier => _$this._clientVerifier;
  set clientVerifier(String? clientVerifier) =>
      _$this._clientVerifier = clientVerifier;

  IdentityLinkCompleteBuilder() {
    IdentityLinkComplete._defaults(this);
  }

  IdentityLinkCompleteBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _loginCode = $v.loginCode;
      _clientVerifier = $v.clientVerifier;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityLinkComplete other) {
    _$v = other as _$IdentityLinkComplete;
  }

  @override
  void update(void Function(IdentityLinkCompleteBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityLinkComplete build() => _build();

  _$IdentityLinkComplete _build() {
    final _$result =
        _$v ??
        _$IdentityLinkComplete._(
          loginCode: BuiltValueNullFieldError.checkNotNull(
            loginCode,
            r'IdentityLinkComplete',
            'loginCode',
          ),
          clientVerifier: BuiltValueNullFieldError.checkNotNull(
            clientVerifier,
            r'IdentityLinkComplete',
            'clientVerifier',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
