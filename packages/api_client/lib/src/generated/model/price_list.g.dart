// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'price_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PriceList extends PriceList {
  @override
  final BuiltList<Price> items;

  factory _$PriceList([void Function(PriceListBuilder)? updates]) =>
      (PriceListBuilder()..update(updates))._build();

  _$PriceList._({required this.items}) : super._();
  @override
  PriceList rebuild(void Function(PriceListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PriceListBuilder toBuilder() => PriceListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PriceList && items == other.items;
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
      r'PriceList',
    )..add('items', items)).toString();
  }
}

class PriceListBuilder implements Builder<PriceList, PriceListBuilder> {
  _$PriceList? _$v;

  ListBuilder<Price>? _items;
  ListBuilder<Price> get items => _$this._items ??= ListBuilder<Price>();
  set items(ListBuilder<Price>? items) => _$this._items = items;

  PriceListBuilder() {
    PriceList._defaults(this);
  }

  PriceListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PriceList other) {
    _$v = other as _$PriceList;
  }

  @override
  void update(void Function(PriceListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PriceList build() => _build();

  _$PriceList _build() {
    _$PriceList _$result;
    try {
      _$result = _$v ?? _$PriceList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PriceList',
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
