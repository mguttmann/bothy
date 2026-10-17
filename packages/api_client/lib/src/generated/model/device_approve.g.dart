// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device_approve.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DeviceApprove extends DeviceApprove {
  @override
  final String verificationCode;

  factory _$DeviceApprove([void Function(DeviceApproveBuilder)? updates]) =>
      (DeviceApproveBuilder()..update(updates))._build();

  _$DeviceApprove._({required this.verificationCode}) : super._();
  @override
  DeviceApprove rebuild(void Function(DeviceApproveBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DeviceApproveBuilder toBuilder() => DeviceApproveBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DeviceApprove && verificationCode == other.verificationCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, verificationCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'DeviceApprove',
    )..add('verificationCode', verificationCode)).toString();
  }
}

class DeviceApproveBuilder
    implements Builder<DeviceApprove, DeviceApproveBuilder> {
  _$DeviceApprove? _$v;

  String? _verificationCode;
  String? get verificationCode => _$this._verificationCode;
  set verificationCode(String? verificationCode) =>
      _$this._verificationCode = verificationCode;

  DeviceApproveBuilder() {
    DeviceApprove._defaults(this);
  }

  DeviceApproveBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _verificationCode = $v.verificationCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DeviceApprove other) {
    _$v = other as _$DeviceApprove;
  }

  @override
  void update(void Function(DeviceApproveBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DeviceApprove build() => _build();

  _$DeviceApprove _build() {
    final _$result =
        _$v ??
        _$DeviceApprove._(
          verificationCode: BuiltValueNullFieldError.checkNotNull(
            verificationCode,
            r'DeviceApprove',
            'verificationCode',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
