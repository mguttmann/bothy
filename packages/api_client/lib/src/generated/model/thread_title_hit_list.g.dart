// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_title_hit_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadTitleHitList extends ThreadTitleHitList {
  @override
  final BuiltList<ThreadTitleHitListItemsInner> items;

  factory _$ThreadTitleHitList([
    void Function(ThreadTitleHitListBuilder)? updates,
  ]) => (ThreadTitleHitListBuilder()..update(updates))._build();

  _$ThreadTitleHitList._({required this.items}) : super._();
  @override
  ThreadTitleHitList rebuild(
    void Function(ThreadTitleHitListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ThreadTitleHitListBuilder toBuilder() =>
      ThreadTitleHitListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadTitleHitList && items == other.items;
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
      r'ThreadTitleHitList',
    )..add('items', items)).toString();
  }
}

class ThreadTitleHitListBuilder
    implements Builder<ThreadTitleHitList, ThreadTitleHitListBuilder> {
  _$ThreadTitleHitList? _$v;

  ListBuilder<ThreadTitleHitListItemsInner>? _items;
  ListBuilder<ThreadTitleHitListItemsInner> get items =>
      _$this._items ??= ListBuilder<ThreadTitleHitListItemsInner>();
  set items(ListBuilder<ThreadTitleHitListItemsInner>? items) =>
      _$this._items = items;

  ThreadTitleHitListBuilder() {
    ThreadTitleHitList._defaults(this);
  }

  ThreadTitleHitListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadTitleHitList other) {
    _$v = other as _$ThreadTitleHitList;
  }

  @override
  void update(void Function(ThreadTitleHitListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadTitleHitList build() => _build();

  _$ThreadTitleHitList _build() {
    _$ThreadTitleHitList _$result;
    try {
      _$result = _$v ?? _$ThreadTitleHitList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadTitleHitList',
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
