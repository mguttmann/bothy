// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_version_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionVersionPage extends AgentDefinitionVersionPage {
  @override
  final BuiltList<AgentDefinitionVersion> items;
  @override
  final String? nextCursor;

  factory _$AgentDefinitionVersionPage([
    void Function(AgentDefinitionVersionPageBuilder)? updates,
  ]) => (AgentDefinitionVersionPageBuilder()..update(updates))._build();

  _$AgentDefinitionVersionPage._({required this.items, this.nextCursor})
    : super._();
  @override
  AgentDefinitionVersionPage rebuild(
    void Function(AgentDefinitionVersionPageBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionVersionPageBuilder toBuilder() =>
      AgentDefinitionVersionPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionVersionPage &&
        items == other.items &&
        nextCursor == other.nextCursor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionVersionPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class AgentDefinitionVersionPageBuilder
    implements
        Builder<AgentDefinitionVersionPage, AgentDefinitionVersionPageBuilder> {
  _$AgentDefinitionVersionPage? _$v;

  ListBuilder<AgentDefinitionVersion>? _items;
  ListBuilder<AgentDefinitionVersion> get items =>
      _$this._items ??= ListBuilder<AgentDefinitionVersion>();
  set items(ListBuilder<AgentDefinitionVersion>? items) =>
      _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  AgentDefinitionVersionPageBuilder() {
    AgentDefinitionVersionPage._defaults(this);
  }

  AgentDefinitionVersionPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionVersionPage other) {
    _$v = other as _$AgentDefinitionVersionPage;
  }

  @override
  void update(void Function(AgentDefinitionVersionPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionVersionPage build() => _build();

  _$AgentDefinitionVersionPage _build() {
    _$AgentDefinitionVersionPage _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinitionVersionPage._(
            items: items.build(),
            nextCursor: nextCursor,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinitionVersionPage',
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
