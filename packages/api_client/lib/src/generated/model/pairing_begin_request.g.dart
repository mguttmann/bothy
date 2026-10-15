// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pairing_begin_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PairingBeginRequest extends PairingBeginRequest {
  @override
  final DeviceRegistration device;

  factory _$PairingBeginRequest([
    void Function(PairingBeginRequestBuilder)? updates,
  ]) => (PairingBeginRequestBuilder()..update(updates))._build();

  _$PairingBeginRequest._({required this.device}) : super._();
  @override
  PairingBeginRequest rebuild(
    void Function(PairingBeginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PairingBeginRequestBuilder toBuilder() =>
      PairingBeginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PairingBeginRequest && device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PairingBeginRequest',
    )..add('device', device)).toString();
  }
}

class PairingBeginRequestBuilder
    implements Builder<PairingBeginRequest, PairingBeginRequestBuilder> {
  _$PairingBeginRequest? _$v;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  PairingBeginRequestBuilder() {
    PairingBeginRequest._defaults(this);
  }

  PairingBeginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _device = $v.device.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PairingBeginRequest other) {
    _$v = other as _$PairingBeginRequest;
  }

  @override
  void update(void Function(PairingBeginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PairingBeginRequest build() => _build();

  _$PairingBeginRequest _build() {
    _$PairingBeginRequest _$result;
    try {
      _$result = _$v ?? _$PairingBeginRequest._(device: device.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'device';
        device.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PairingBeginRequest',
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
