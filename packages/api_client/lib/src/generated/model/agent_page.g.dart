// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentPage extends AgentPage {
  @override
  final BuiltList<Agent> items;
  @override
  final String? nextCursor;

  factory _$AgentPage([void Function(AgentPageBuilder)? updates]) =>
      (AgentPageBuilder()..update(updates))._build();

  _$AgentPage._({required this.items, this.nextCursor}) : super._();
  @override
  AgentPage rebuild(void Function(AgentPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentPageBuilder toBuilder() => AgentPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentPage &&
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
    return (newBuiltValueToStringHelper(r'AgentPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class AgentPageBuilder implements Builder<AgentPage, AgentPageBuilder> {
  _$AgentPage? _$v;

  ListBuilder<Agent>? _items;
  ListBuilder<Agent> get items => _$this._items ??= ListBuilder<Agent>();
  set items(ListBuilder<Agent>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  AgentPageBuilder() {
    AgentPage._defaults(this);
  }

  AgentPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentPage other) {
    _$v = other as _$AgentPage;
  }

  @override
  void update(void Function(AgentPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentPage build() => _build();

  _$AgentPage _build() {
    _$AgentPage _$result;
    try {
      _$result =
          _$v ?? _$AgentPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentPage',
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
