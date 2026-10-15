// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_settings_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelSettingsChangedPayload extends ChannelSettingsChangedPayload {
  @override
  final String channelId;
  @override
  final int settingsVersion;

  factory _$ChannelSettingsChangedPayload([
    void Function(ChannelSettingsChangedPayloadBuilder)? updates,
  ]) => (ChannelSettingsChangedPayloadBuilder()..update(updates))._build();

  _$ChannelSettingsChangedPayload._({
    required this.channelId,
    required this.settingsVersion,
  }) : super._();
  @override
  ChannelSettingsChangedPayload rebuild(
    void Function(ChannelSettingsChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ChannelSettingsChangedPayloadBuilder toBuilder() =>
      ChannelSettingsChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelSettingsChangedPayload &&
        channelId == other.channelId &&
        settingsVersion == other.settingsVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, settingsVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelSettingsChangedPayload')
          ..add('channelId', channelId)
          ..add('settingsVersion', settingsVersion))
        .toString();
  }
}

class ChannelSettingsChangedPayloadBuilder
    implements
        Builder<
          ChannelSettingsChangedPayload,
          ChannelSettingsChangedPayloadBuilder
        > {
  _$ChannelSettingsChangedPayload? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _settingsVersion;
  int? get settingsVersion => _$this._settingsVersion;
  set settingsVersion(int? settingsVersion) =>
      _$this._settingsVersion = settingsVersion;

  ChannelSettingsChangedPayloadBuilder() {
    ChannelSettingsChangedPayload._defaults(this);
  }

  ChannelSettingsChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _settingsVersion = $v.settingsVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelSettingsChangedPayload other) {
    _$v = other as _$ChannelSettingsChangedPayload;
  }

  @override
  void update(void Function(ChannelSettingsChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelSettingsChangedPayload build() => _build();

  _$ChannelSettingsChangedPayload _build() {
    final _$result =
        _$v ??
        _$ChannelSettingsChangedPayload._(
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ChannelSettingsChangedPayload',
            'channelId',
          ),
          settingsVersion: BuiltValueNullFieldError.checkNotNull(
            settingsVersion,
            r'ChannelSettingsChangedPayload',
            'settingsVersion',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
