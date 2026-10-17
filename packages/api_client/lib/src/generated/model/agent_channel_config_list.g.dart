// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_channel_config_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentChannelConfigList extends AgentChannelConfigList {
  @override
  final BuiltList<AgentChannelConfig> items;

  factory _$AgentChannelConfigList([
    void Function(AgentChannelConfigListBuilder)? updates,
  ]) => (AgentChannelConfigListBuilder()..update(updates))._build();

  _$AgentChannelConfigList._({required this.items}) : super._();
  @override
  AgentChannelConfigList rebuild(
    void Function(AgentChannelConfigListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentChannelConfigListBuilder toBuilder() =>
      AgentChannelConfigListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentChannelConfigList && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AgentChannelConfigList',
    )..add('items', items)).toString();
  }
}

class AgentChannelConfigListBuilder
    implements Builder<AgentChannelConfigList, AgentChannelConfigListBuilder> {
  _$AgentChannelConfigList? _$v;

  ListBuilder<AgentChannelConfig>? _items;
  ListBuilder<AgentChannelConfig> get items =>
      _$this._items ??= ListBuilder<AgentChannelConfig>();
  set items(ListBuilder<AgentChannelConfig>? items) => _$this._items = items;

  AgentChannelConfigListBuilder() {
    AgentChannelConfigList._defaults(this);
  }

  AgentChannelConfigListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentChannelConfigList other) {
    _$v = other as _$AgentChannelConfigList;
  }

  @override
  void update(void Function(AgentChannelConfigListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentChannelConfigList build() => _build();

  _$AgentChannelConfigList _build() {
    _$AgentChannelConfigList _$result;
    try {
      _$result = _$v ?? _$AgentChannelConfigList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentChannelConfigList',
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
