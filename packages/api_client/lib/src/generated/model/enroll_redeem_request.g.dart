// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'enroll_redeem_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EnrollRedeemRequest extends EnrollRedeemRequest {
  @override
  final String enrollmentCode;
  @override
  final DeviceRegistration device;

  factory _$EnrollRedeemRequest([
    void Function(EnrollRedeemRequestBuilder)? updates,
  ]) => (EnrollRedeemRequestBuilder()..update(updates))._build();

  _$EnrollRedeemRequest._({required this.enrollmentCode, required this.device})
    : super._();
  @override
  EnrollRedeemRequest rebuild(
    void Function(EnrollRedeemRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  EnrollRedeemRequestBuilder toBuilder() =>
      EnrollRedeemRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EnrollRedeemRequest &&
        enrollmentCode == other.enrollmentCode &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, enrollmentCode.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EnrollRedeemRequest')
          ..add('enrollmentCode', enrollmentCode)
          ..add('device', device))
        .toString();
  }
}

class EnrollRedeemRequestBuilder
    implements Builder<EnrollRedeemRequest, EnrollRedeemRequestBuilder> {
  _$EnrollRedeemRequest? _$v;

  String? _enrollmentCode;
  String? get enrollmentCode => _$this._enrollmentCode;
  set enrollmentCode(String? enrollmentCode) =>
      _$this._enrollmentCode = enrollmentCode;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  EnrollRedeemRequestBuilder() {
    EnrollRedeemRequest._defaults(this);
  }

  EnrollRedeemRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _enrollmentCode = $v.enrollmentCode;
      _device = $v.device.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EnrollRedeemRequest other) {
    _$v = other as _$EnrollRedeemRequest;
  }

  @override
  void update(void Function(EnrollRedeemRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EnrollRedeemRequest build() => _build();

  _$EnrollRedeemRequest _build() {
    _$EnrollRedeemRequest _$result;
    try {
      _$result =
          _$v ??
          _$EnrollRedeemRequest._(
            enrollmentCode: BuiltValueNullFieldError.checkNotNull(
              enrollmentCode,
              r'EnrollRedeemRequest',
              'enrollmentCode',
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
          r'EnrollRedeemRequest',
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
