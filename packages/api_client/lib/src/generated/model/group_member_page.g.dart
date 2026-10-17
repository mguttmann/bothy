// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_member_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupMemberPage extends GroupMemberPage {
  @override
  final BuiltList<GroupMember> items;
  @override
  final String? nextCursor;

  factory _$GroupMemberPage([void Function(GroupMemberPageBuilder)? updates]) =>
      (GroupMemberPageBuilder()..update(updates))._build();

  _$GroupMemberPage._({required this.items, this.nextCursor}) : super._();
  @override
  GroupMemberPage rebuild(void Function(GroupMemberPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupMemberPageBuilder toBuilder() => GroupMemberPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupMemberPage &&
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
    return (newBuiltValueToStringHelper(r'GroupMemberPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class GroupMemberPageBuilder
    implements Builder<GroupMemberPage, GroupMemberPageBuilder> {
  _$GroupMemberPage? _$v;

  ListBuilder<GroupMember>? _items;
  ListBuilder<GroupMember> get items =>
      _$this._items ??= ListBuilder<GroupMember>();
  set items(ListBuilder<GroupMember>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  GroupMemberPageBuilder() {
    GroupMemberPage._defaults(this);
  }

  GroupMemberPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupMemberPage other) {
    _$v = other as _$GroupMemberPage;
  }

  @override
  void update(void Function(GroupMemberPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupMemberPage build() => _build();

  _$GroupMemberPage _build() {
    _$GroupMemberPage _$result;
    try {
      _$result =
          _$v ??
          _$GroupMemberPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GroupMemberPage',
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
