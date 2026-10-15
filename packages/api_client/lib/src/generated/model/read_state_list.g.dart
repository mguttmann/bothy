// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'read_state_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReadStateList extends ReadStateList {
  @override
  final BuiltList<ReadState> items;

  factory _$ReadStateList([void Function(ReadStateListBuilder)? updates]) =>
      (ReadStateListBuilder()..update(updates))._build();

  _$ReadStateList._({required this.items}) : super._();
  @override
  ReadStateList rebuild(void Function(ReadStateListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReadStateListBuilder toBuilder() => ReadStateListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReadStateList && items == other.items;
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
      r'ReadStateList',
    )..add('items', items)).toString();
  }
}

class ReadStateListBuilder
    implements Builder<ReadStateList, ReadStateListBuilder> {
  _$ReadStateList? _$v;

  ListBuilder<ReadState>? _items;
  ListBuilder<ReadState> get items =>
      _$this._items ??= ListBuilder<ReadState>();
  set items(ListBuilder<ReadState>? items) => _$this._items = items;

  ReadStateListBuilder() {
    ReadStateList._defaults(this);
  }

  ReadStateListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReadStateList other) {
    _$v = other as _$ReadStateList;
  }

  @override
  void update(void Function(ReadStateListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReadStateList build() => _build();

  _$ReadStateList _build() {
    _$ReadStateList _$result;
    try {
      _$result = _$v ?? _$ReadStateList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ReadStateList',
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
