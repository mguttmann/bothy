// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_tag_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ForumTagList extends ForumTagList {
  @override
  final BuiltList<ForumTag> items;

  factory _$ForumTagList([void Function(ForumTagListBuilder)? updates]) =>
      (ForumTagListBuilder()..update(updates))._build();

  _$ForumTagList._({required this.items}) : super._();
  @override
  ForumTagList rebuild(void Function(ForumTagListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ForumTagListBuilder toBuilder() => ForumTagListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ForumTagList && items == other.items;
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
      r'ForumTagList',
    )..add('items', items)).toString();
  }
}

class ForumTagListBuilder
    implements Builder<ForumTagList, ForumTagListBuilder> {
  _$ForumTagList? _$v;

  ListBuilder<ForumTag>? _items;
  ListBuilder<ForumTag> get items => _$this._items ??= ListBuilder<ForumTag>();
  set items(ListBuilder<ForumTag>? items) => _$this._items = items;

  ForumTagListBuilder() {
    ForumTagList._defaults(this);
  }

  ForumTagListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ForumTagList other) {
    _$v = other as _$ForumTagList;
  }

  @override
  void update(void Function(ForumTagListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ForumTagList build() => _build();

  _$ForumTagList _build() {
    _$ForumTagList _$result;
    try {
      _$result = _$v ?? _$ForumTagList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ForumTagList',
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
