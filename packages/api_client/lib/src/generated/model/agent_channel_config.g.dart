// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_channel_config.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentChannelConfig extends AgentChannelConfig {
  @override
  final String channelId;
  @override
  final String? spaceId;
  @override
  final ChannelBehavior behavior;

  factory _$AgentChannelConfig([
    void Function(AgentChannelConfigBuilder)? updates,
  ]) => (AgentChannelConfigBuilder()..update(updates))._build();

  _$AgentChannelConfig._({
    required this.channelId,
    this.spaceId,
    required this.behavior,
  }) : super._();
  @override
  AgentChannelConfig rebuild(
    void Function(AgentChannelConfigBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentChannelConfigBuilder toBuilder() =>
      AgentChannelConfigBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentChannelConfig &&
        channelId == other.channelId &&
        spaceId == other.spaceId &&
        behavior == other.behavior;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, behavior.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentChannelConfig')
          ..add('channelId', channelId)
          ..add('spaceId', spaceId)
          ..add('behavior', behavior))
        .toString();
  }
}

class AgentChannelConfigBuilder
    implements Builder<AgentChannelConfig, AgentChannelConfigBuilder> {
  _$AgentChannelConfig? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  ChannelBehaviorBuilder? _behavior;
  ChannelBehaviorBuilder get behavior =>
      _$this._behavior ??= ChannelBehaviorBuilder();
  set behavior(ChannelBehaviorBuilder? behavior) => _$this._behavior = behavior;

  AgentChannelConfigBuilder() {
    AgentChannelConfig._defaults(this);
  }

  AgentChannelConfigBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _spaceId = $v.spaceId;
      _behavior = $v.behavior.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentChannelConfig other) {
    _$v = other as _$AgentChannelConfig;
  }

  @override
  void update(void Function(AgentChannelConfigBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentChannelConfig build() => _build();

  _$AgentChannelConfig _build() {
    _$AgentChannelConfig _$result;
    try {
      _$result =
          _$v ??
          _$AgentChannelConfig._(
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'AgentChannelConfig',
              'channelId',
            ),
            spaceId: spaceId,
            behavior: behavior.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'behavior';
        behavior.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentChannelConfig',
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
