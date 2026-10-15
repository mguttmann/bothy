// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftList extends DraftList {
  @override
  final BuiltList<Draft> items;

  factory _$DraftList([void Function(DraftListBuilder)? updates]) =>
      (DraftListBuilder()..update(updates))._build();

  _$DraftList._({required this.items}) : super._();
  @override
  DraftList rebuild(void Function(DraftListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftListBuilder toBuilder() => DraftListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftList && items == other.items;
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
      r'DraftList',
    )..add('items', items)).toString();
  }
}

class DraftListBuilder implements Builder<DraftList, DraftListBuilder> {
  _$DraftList? _$v;

  ListBuilder<Draft>? _items;
  ListBuilder<Draft> get items => _$this._items ??= ListBuilder<Draft>();
  set items(ListBuilder<Draft>? items) => _$this._items = items;

  DraftListBuilder() {
    DraftList._defaults(this);
  }

  DraftListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftList other) {
    _$v = other as _$DraftList;
  }

  @override
  void update(void Function(DraftListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftList build() => _build();

  _$DraftList _build() {
    _$DraftList _$result;
    try {
      _$result = _$v ?? _$DraftList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DraftList',
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
