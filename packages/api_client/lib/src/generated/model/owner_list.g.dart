// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'owner_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OwnerList extends OwnerList {
  @override
  final BuiltList<Owner> items;

  factory _$OwnerList([void Function(OwnerListBuilder)? updates]) =>
      (OwnerListBuilder()..update(updates))._build();

  _$OwnerList._({required this.items}) : super._();
  @override
  OwnerList rebuild(void Function(OwnerListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OwnerListBuilder toBuilder() => OwnerListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OwnerList && items == other.items;
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
      r'OwnerList',
    )..add('items', items)).toString();
  }
}

class OwnerListBuilder implements Builder<OwnerList, OwnerListBuilder> {
  _$OwnerList? _$v;

  ListBuilder<Owner>? _items;
  ListBuilder<Owner> get items => _$this._items ??= ListBuilder<Owner>();
  set items(ListBuilder<Owner>? items) => _$this._items = items;

  OwnerListBuilder() {
    OwnerList._defaults(this);
  }

  OwnerListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OwnerList other) {
    _$v = other as _$OwnerList;
  }

  @override
  void update(void Function(OwnerListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OwnerList build() => _build();

  _$OwnerList _build() {
    _$OwnerList _$result;
    try {
      _$result = _$v ?? _$OwnerList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'OwnerList',
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
