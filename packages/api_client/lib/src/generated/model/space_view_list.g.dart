// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space_view_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SpaceViewList extends SpaceViewList {
  @override
  final BuiltList<SpaceView> items;

  factory _$SpaceViewList([void Function(SpaceViewListBuilder)? updates]) =>
      (SpaceViewListBuilder()..update(updates))._build();

  _$SpaceViewList._({required this.items}) : super._();
  @override
  SpaceViewList rebuild(void Function(SpaceViewListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpaceViewListBuilder toBuilder() => SpaceViewListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SpaceViewList && items == other.items;
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
      r'SpaceViewList',
    )..add('items', items)).toString();
  }
}

class SpaceViewListBuilder
    implements Builder<SpaceViewList, SpaceViewListBuilder> {
  _$SpaceViewList? _$v;

  ListBuilder<SpaceView>? _items;
  ListBuilder<SpaceView> get items =>
      _$this._items ??= ListBuilder<SpaceView>();
  set items(ListBuilder<SpaceView>? items) => _$this._items = items;

  SpaceViewListBuilder() {
    SpaceViewList._defaults(this);
  }

  SpaceViewListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SpaceViewList other) {
    _$v = other as _$SpaceViewList;
  }

  @override
  void update(void Function(SpaceViewListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SpaceViewList build() => _build();

  _$SpaceViewList _build() {
    _$SpaceViewList _$result;
    try {
      _$result = _$v ?? _$SpaceViewList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SpaceViewList',
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
