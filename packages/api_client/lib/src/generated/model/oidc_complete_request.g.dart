// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'oidc_complete_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OidcCompleteRequest extends OidcCompleteRequest {
  @override
  final String loginCode;
  @override
  final String clientVerifier;
  @override
  final DeviceRegistration device;

  factory _$OidcCompleteRequest([
    void Function(OidcCompleteRequestBuilder)? updates,
  ]) => (OidcCompleteRequestBuilder()..update(updates))._build();

  _$OidcCompleteRequest._({
    required this.loginCode,
    required this.clientVerifier,
    required this.device,
  }) : super._();
  @override
  OidcCompleteRequest rebuild(
    void Function(OidcCompleteRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  OidcCompleteRequestBuilder toBuilder() =>
      OidcCompleteRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OidcCompleteRequest &&
        loginCode == other.loginCode &&
        clientVerifier == other.clientVerifier &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, loginCode.hashCode);
    _$hash = $jc(_$hash, clientVerifier.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'OidcCompleteRequest')
          ..add('loginCode', loginCode)
          ..add('clientVerifier', clientVerifier)
          ..add('device', device))
        .toString();
  }
}

class OidcCompleteRequestBuilder
    implements Builder<OidcCompleteRequest, OidcCompleteRequestBuilder> {
  _$OidcCompleteRequest? _$v;

  String? _loginCode;
  String? get loginCode => _$this._loginCode;
  set loginCode(String? loginCode) => _$this._loginCode = loginCode;

  String? _clientVerifier;
  String? get clientVerifier => _$this._clientVerifier;
  set clientVerifier(String? clientVerifier) =>
      _$this._clientVerifier = clientVerifier;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  OidcCompleteRequestBuilder() {
    OidcCompleteRequest._defaults(this);
  }

  OidcCompleteRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _loginCode = $v.loginCode;
      _clientVerifier = $v.clientVerifier;
      _device = $v.device.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OidcCompleteRequest other) {
    _$v = other as _$OidcCompleteRequest;
  }

  @override
  void update(void Function(OidcCompleteRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OidcCompleteRequest build() => _build();

  _$OidcCompleteRequest _build() {
    _$OidcCompleteRequest _$result;
    try {
      _$result =
          _$v ??
          _$OidcCompleteRequest._(
            loginCode: BuiltValueNullFieldError.checkNotNull(
              loginCode,
              r'OidcCompleteRequest',
              'loginCode',
            ),
            clientVerifier: BuiltValueNullFieldError.checkNotNull(
              clientVerifier,
              r'OidcCompleteRequest',
              'clientVerifier',
            ),
            device: device.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'device';
        device.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'OidcCompleteRequest',
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
