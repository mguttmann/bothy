// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_config_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentConfigChangedPayload extends AgentConfigChangedPayload {
  @override
  final String channelId;
  @override
  final int settingsVersion;

  factory _$AgentConfigChangedPayload([
    void Function(AgentConfigChangedPayloadBuilder)? updates,
  ]) => (AgentConfigChangedPayloadBuilder()..update(updates))._build();

  _$AgentConfigChangedPayload._({
    required this.channelId,
    required this.settingsVersion,
  }) : super._();
  @override
  AgentConfigChangedPayload rebuild(
    void Function(AgentConfigChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentConfigChangedPayloadBuilder toBuilder() =>
      AgentConfigChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentConfigChangedPayload &&
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
    return (newBuiltValueToStringHelper(r'AgentConfigChangedPayload')
          ..add('channelId', channelId)
          ..add('settingsVersion', settingsVersion))
        .toString();
  }
}

class AgentConfigChangedPayloadBuilder
    implements
        Builder<AgentConfigChangedPayload, AgentConfigChangedPayloadBuilder> {
  _$AgentConfigChangedPayload? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _settingsVersion;
  int? get settingsVersion => _$this._settingsVersion;
  set settingsVersion(int? settingsVersion) =>
      _$this._settingsVersion = settingsVersion;

  AgentConfigChangedPayloadBuilder() {
    AgentConfigChangedPayload._defaults(this);
  }

  AgentConfigChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _settingsVersion = $v.settingsVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentConfigChangedPayload other) {
    _$v = other as _$AgentConfigChangedPayload;
  }

  @override
  void update(void Function(AgentConfigChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentConfigChangedPayload build() => _build();

  _$AgentConfigChangedPayload _build() {
    final _$result =
        _$v ??
        _$AgentConfigChangedPayload._(
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'AgentConfigChangedPayload',
            'channelId',
          ),
          settingsVersion: BuiltValueNullFieldError.checkNotNull(
            settingsVersion,
            r'AgentConfigChangedPayload',
            'settingsVersion',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
