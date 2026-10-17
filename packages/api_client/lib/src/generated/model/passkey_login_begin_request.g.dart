// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'passkey_login_begin_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PasskeyLoginBeginRequest extends PasskeyLoginBeginRequest {
  @override
  final String? loginName;

  factory _$PasskeyLoginBeginRequest([
    void Function(PasskeyLoginBeginRequestBuilder)? updates,
  ]) => (PasskeyLoginBeginRequestBuilder()..update(updates))._build();

  _$PasskeyLoginBeginRequest._({this.loginName}) : super._();
  @override
  PasskeyLoginBeginRequest rebuild(
    void Function(PasskeyLoginBeginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PasskeyLoginBeginRequestBuilder toBuilder() =>
      PasskeyLoginBeginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PasskeyLoginBeginRequest && loginName == other.loginName;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, loginName.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PasskeyLoginBeginRequest',
    )..add('loginName', loginName)).toString();
  }
}

class PasskeyLoginBeginRequestBuilder
    implements
        Builder<PasskeyLoginBeginRequest, PasskeyLoginBeginRequestBuilder> {
  _$PasskeyLoginBeginRequest? _$v;

  String? _loginName;
  String? get loginName => _$this._loginName;
  set loginName(String? loginName) => _$this._loginName = loginName;

  PasskeyLoginBeginRequestBuilder() {
    PasskeyLoginBeginRequest._defaults(this);
  }

  PasskeyLoginBeginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _loginName = $v.loginName;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PasskeyLoginBeginRequest other) {
    _$v = other as _$PasskeyLoginBeginRequest;
  }

  @override
  void update(void Function(PasskeyLoginBeginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PasskeyLoginBeginRequest build() => _build();

  _$PasskeyLoginBeginRequest _build() {
    final _$result = _$v ?? _$PasskeyLoginBeginRequest._(loginName: loginName);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
