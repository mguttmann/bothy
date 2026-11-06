// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_draft_channel.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDraftChannel extends AgentDraftChannel {
  @override
  final String spaceName;
  @override
  final String channelName;

  factory _$AgentDraftChannel([
    void Function(AgentDraftChannelBuilder)? updates,
  ]) => (AgentDraftChannelBuilder()..update(updates))._build();

  _$AgentDraftChannel._({required this.spaceName, required this.channelName})
    : super._();
  @override
  AgentDraftChannel rebuild(void Function(AgentDraftChannelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentDraftChannelBuilder toBuilder() =>
      AgentDraftChannelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDraftChannel &&
        spaceName == other.spaceName &&
        channelName == other.channelName;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, spaceName.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDraftChannel')
          ..add('spaceName', spaceName)
          ..add('channelName', channelName))
        .toString();
  }
}

class AgentDraftChannelBuilder
    implements Builder<AgentDraftChannel, AgentDraftChannelBuilder> {
  _$AgentDraftChannel? _$v;

  String? _spaceName;
  String? get spaceName => _$this._spaceName;
  set spaceName(String? spaceName) => _$this._spaceName = spaceName;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  AgentDraftChannelBuilder() {
    AgentDraftChannel._defaults(this);
  }

  AgentDraftChannelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _spaceName = $v.spaceName;
      _channelName = $v.channelName;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDraftChannel other) {
    _$v = other as _$AgentDraftChannel;
  }

  @override
  void update(void Function(AgentDraftChannelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDraftChannel build() => _build();

  _$AgentDraftChannel _build() {
    final _$result =
        _$v ??
        _$AgentDraftChannel._(
          spaceName: BuiltValueNullFieldError.checkNotNull(
            spaceName,
            r'AgentDraftChannel',
            'spaceName',
          ),
          channelName: BuiltValueNullFieldError.checkNotNull(
            channelName,
            r'AgentDraftChannel',
            'channelName',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
