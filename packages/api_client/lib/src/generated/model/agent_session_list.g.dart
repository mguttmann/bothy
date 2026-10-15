// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentSessionList extends AgentSessionList {
  @override
  final BuiltList<AgentSession> items;

  factory _$AgentSessionList([
    void Function(AgentSessionListBuilder)? updates,
  ]) => (AgentSessionListBuilder()..update(updates))._build();

  _$AgentSessionList._({required this.items}) : super._();
  @override
  AgentSessionList rebuild(void Function(AgentSessionListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentSessionListBuilder toBuilder() =>
      AgentSessionListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentSessionList && items == other.items;
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
      r'AgentSessionList',
    )..add('items', items)).toString();
  }
}

class AgentSessionListBuilder
    implements Builder<AgentSessionList, AgentSessionListBuilder> {
  _$AgentSessionList? _$v;

  ListBuilder<AgentSession>? _items;
  ListBuilder<AgentSession> get items =>
      _$this._items ??= ListBuilder<AgentSession>();
  set items(ListBuilder<AgentSession>? items) => _$this._items = items;

  AgentSessionListBuilder() {
    AgentSessionList._defaults(this);
  }

  AgentSessionListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentSessionList other) {
    _$v = other as _$AgentSessionList;
  }

  @override
  void update(void Function(AgentSessionListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentSessionList build() => _build();

  _$AgentSessionList _build() {
    _$AgentSessionList _$result;
    try {
      _$result = _$v ?? _$AgentSessionList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentSessionList',
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
