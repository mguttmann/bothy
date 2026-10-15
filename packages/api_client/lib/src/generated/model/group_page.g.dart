// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupPage extends GroupPage {
  @override
  final BuiltList<Group> items;
  @override
  final String? nextCursor;

  factory _$GroupPage([void Function(GroupPageBuilder)? updates]) =>
      (GroupPageBuilder()..update(updates))._build();

  _$GroupPage._({required this.items, this.nextCursor}) : super._();
  @override
  GroupPage rebuild(void Function(GroupPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupPageBuilder toBuilder() => GroupPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupPage &&
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
    return (newBuiltValueToStringHelper(r'GroupPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class GroupPageBuilder implements Builder<GroupPage, GroupPageBuilder> {
  _$GroupPage? _$v;

  ListBuilder<Group>? _items;
  ListBuilder<Group> get items => _$this._items ??= ListBuilder<Group>();
  set items(ListBuilder<Group>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  GroupPageBuilder() {
    GroupPage._defaults(this);
  }

  GroupPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupPage other) {
    _$v = other as _$GroupPage;
  }

  @override
  void update(void Function(GroupPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupPage build() => _build();

  _$GroupPage _build() {
    _$GroupPage _$result;
    try {
      _$result =
          _$v ?? _$GroupPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GroupPage',
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
