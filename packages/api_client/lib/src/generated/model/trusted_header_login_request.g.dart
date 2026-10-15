// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'trusted_header_login_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TrustedHeaderLoginRequest extends TrustedHeaderLoginRequest {
  @override
  final DeviceRegistration device;
  @override
  final String? enrollmentCode;

  factory _$TrustedHeaderLoginRequest([
    void Function(TrustedHeaderLoginRequestBuilder)? updates,
  ]) => (TrustedHeaderLoginRequestBuilder()..update(updates))._build();

  _$TrustedHeaderLoginRequest._({required this.device, this.enrollmentCode})
    : super._();
  @override
  TrustedHeaderLoginRequest rebuild(
    void Function(TrustedHeaderLoginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TrustedHeaderLoginRequestBuilder toBuilder() =>
      TrustedHeaderLoginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TrustedHeaderLoginRequest &&
        device == other.device &&
        enrollmentCode == other.enrollmentCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jc(_$hash, enrollmentCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TrustedHeaderLoginRequest')
          ..add('device', device)
          ..add('enrollmentCode', enrollmentCode))
        .toString();
  }
}

class TrustedHeaderLoginRequestBuilder
    implements
        Builder<TrustedHeaderLoginRequest, TrustedHeaderLoginRequestBuilder> {
  _$TrustedHeaderLoginRequest? _$v;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  String? _enrollmentCode;
  String? get enrollmentCode => _$this._enrollmentCode;
  set enrollmentCode(String? enrollmentCode) =>
      _$this._enrollmentCode = enrollmentCode;

  TrustedHeaderLoginRequestBuilder() {
    TrustedHeaderLoginRequest._defaults(this);
  }

  TrustedHeaderLoginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _device = $v.device.toBuilder();
      _enrollmentCode = $v.enrollmentCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TrustedHeaderLoginRequest other) {
    _$v = other as _$TrustedHeaderLoginRequest;
  }

  @override
  void update(void Function(TrustedHeaderLoginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TrustedHeaderLoginRequest build() => _build();

  _$TrustedHeaderLoginRequest _build() {
    _$TrustedHeaderLoginRequest _$result;
    try {
      _$result =
          _$v ??
          _$TrustedHeaderLoginRequest._(
            device: device.build(),
            enrollmentCode: enrollmentCode,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'device';
        device.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TrustedHeaderLoginRequest',
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
