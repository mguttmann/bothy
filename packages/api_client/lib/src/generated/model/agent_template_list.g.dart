// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_template_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentTemplateList extends AgentTemplateList {
  @override
  final BuiltList<AgentTemplate> items;

  factory _$AgentTemplateList([
    void Function(AgentTemplateListBuilder)? updates,
  ]) => (AgentTemplateListBuilder()..update(updates))._build();

  _$AgentTemplateList._({required this.items}) : super._();
  @override
  AgentTemplateList rebuild(void Function(AgentTemplateListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentTemplateListBuilder toBuilder() =>
      AgentTemplateListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentTemplateList && items == other.items;
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
      r'AgentTemplateList',
    )..add('items', items)).toString();
  }
}

class AgentTemplateListBuilder
    implements Builder<AgentTemplateList, AgentTemplateListBuilder> {
  _$AgentTemplateList? _$v;

  ListBuilder<AgentTemplate>? _items;
  ListBuilder<AgentTemplate> get items =>
      _$this._items ??= ListBuilder<AgentTemplate>();
  set items(ListBuilder<AgentTemplate>? items) => _$this._items = items;

  AgentTemplateListBuilder() {
    AgentTemplateList._defaults(this);
  }

  AgentTemplateListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentTemplateList other) {
    _$v = other as _$AgentTemplateList;
  }

  @override
  void update(void Function(AgentTemplateListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentTemplateList build() => _build();

  _$AgentTemplateList _build() {
    _$AgentTemplateList _$result;
    try {
      _$result = _$v ?? _$AgentTemplateList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentTemplateList',
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
