// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device_pending_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DevicePendingPayload extends DevicePendingPayload {
  @override
  final String deviceId;
  @override
  final DevicePlatform platform;

  factory _$DevicePendingPayload([
    void Function(DevicePendingPayloadBuilder)? updates,
  ]) => (DevicePendingPayloadBuilder()..update(updates))._build();

  _$DevicePendingPayload._({required this.deviceId, required this.platform})
    : super._();
  @override
  DevicePendingPayload rebuild(
    void Function(DevicePendingPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  DevicePendingPayloadBuilder toBuilder() =>
      DevicePendingPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DevicePendingPayload &&
        deviceId == other.deviceId &&
        platform == other.platform;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DevicePendingPayload')
          ..add('deviceId', deviceId)
          ..add('platform', platform))
        .toString();
  }
}

class DevicePendingPayloadBuilder
    implements Builder<DevicePendingPayload, DevicePendingPayloadBuilder> {
  _$DevicePendingPayload? _$v;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  DevicePlatform? _platform;
  DevicePlatform? get platform => _$this._platform;
  set platform(DevicePlatform? platform) => _$this._platform = platform;

  DevicePendingPayloadBuilder() {
    DevicePendingPayload._defaults(this);
  }

  DevicePendingPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _deviceId = $v.deviceId;
      _platform = $v.platform;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DevicePendingPayload other) {
    _$v = other as _$DevicePendingPayload;
  }

  @override
  void update(void Function(DevicePendingPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DevicePendingPayload build() => _build();

  _$DevicePendingPayload _build() {
    final _$result =
        _$v ??
        _$DevicePendingPayload._(
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'DevicePendingPayload',
            'deviceId',
          ),
          platform: BuiltValueNullFieldError.checkNotNull(
            platform,
            r'DevicePendingPayload',
            'platform',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
