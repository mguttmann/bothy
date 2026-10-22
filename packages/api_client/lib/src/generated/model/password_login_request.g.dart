// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'password_login_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PasswordLoginRequest extends PasswordLoginRequest {
  @override
  final LoginClient? client;
  @override
  final String loginName;
  @override
  final String password;
  @override
  final String totpCode;
  @override
  final DeviceRegistration? device;

  factory _$PasswordLoginRequest([
    void Function(PasswordLoginRequestBuilder)? updates,
  ]) => (PasswordLoginRequestBuilder()..update(updates))._build();

  _$PasswordLoginRequest._({
    this.client,
    required this.loginName,
    required this.password,
    required this.totpCode,
    this.device,
  }) : super._();
  @override
  PasswordLoginRequest rebuild(
    void Function(PasswordLoginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PasswordLoginRequestBuilder toBuilder() =>
      PasswordLoginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PasswordLoginRequest &&
        client == other.client &&
        loginName == other.loginName &&
        password == other.password &&
        totpCode == other.totpCode &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, client.hashCode);
    _$hash = $jc(_$hash, loginName.hashCode);
    _$hash = $jc(_$hash, password.hashCode);
    _$hash = $jc(_$hash, totpCode.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PasswordLoginRequest')
          ..add('client', client)
          ..add('loginName', loginName)
          ..add('password', password)
          ..add('totpCode', totpCode)
          ..add('device', device))
        .toString();
  }
}

class PasswordLoginRequestBuilder
    implements Builder<PasswordLoginRequest, PasswordLoginRequestBuilder> {
  _$PasswordLoginRequest? _$v;

  LoginClient? _client;
  LoginClient? get client => _$this._client;
  set client(LoginClient? client) => _$this._client = client;

  String? _loginName;
  String? get loginName => _$this._loginName;
  set loginName(String? loginName) => _$this._loginName = loginName;

  String? _password;
  String? get password => _$this._password;
  set password(String? password) => _$this._password = password;

  String? _totpCode;
  String? get totpCode => _$this._totpCode;
  set totpCode(String? totpCode) => _$this._totpCode = totpCode;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  PasswordLoginRequestBuilder() {
    PasswordLoginRequest._defaults(this);
  }

  PasswordLoginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _client = $v.client;
      _loginName = $v.loginName;
      _password = $v.password;
      _totpCode = $v.totpCode;
      _device = $v.device?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PasswordLoginRequest other) {
    _$v = other as _$PasswordLoginRequest;
  }

  @override
  void update(void Function(PasswordLoginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PasswordLoginRequest build() => _build();

  _$PasswordLoginRequest _build() {
    _$PasswordLoginRequest _$result;
    try {
      _$result =
          _$v ??
          _$PasswordLoginRequest._(
            client: client,
            loginName: BuiltValueNullFieldError.checkNotNull(
              loginName,
              r'PasswordLoginRequest',
              'loginName',
            ),
            password: BuiltValueNullFieldError.checkNotNull(
              password,
              r'PasswordLoginRequest',
              'password',
            ),
            totpCode: BuiltValueNullFieldError.checkNotNull(
              totpCode,
              r'PasswordLoginRequest',
              'totpCode',
            ),
            device: _device?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'device';
        _device?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PasswordLoginRequest',
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
