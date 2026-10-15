// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_view_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelViewList extends ChannelViewList {
  @override
  final BuiltList<ChannelView> items;

  factory _$ChannelViewList([void Function(ChannelViewListBuilder)? updates]) =>
      (ChannelViewListBuilder()..update(updates))._build();

  _$ChannelViewList._({required this.items}) : super._();
  @override
  ChannelViewList rebuild(void Function(ChannelViewListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelViewListBuilder toBuilder() => ChannelViewListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelViewList && items == other.items;
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
      r'ChannelViewList',
    )..add('items', items)).toString();
  }
}

class ChannelViewListBuilder
    implements Builder<ChannelViewList, ChannelViewListBuilder> {
  _$ChannelViewList? _$v;

  ListBuilder<ChannelView>? _items;
  ListBuilder<ChannelView> get items =>
      _$this._items ??= ListBuilder<ChannelView>();
  set items(ListBuilder<ChannelView>? items) => _$this._items = items;

  ChannelViewListBuilder() {
    ChannelViewList._defaults(this);
  }

  ChannelViewListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelViewList other) {
    _$v = other as _$ChannelViewList;
  }

  @override
  void update(void Function(ChannelViewListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelViewList build() => _build();

  _$ChannelViewList _build() {
    _$ChannelViewList _$result;
    try {
      _$result = _$v ?? _$ChannelViewList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ChannelViewList',
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
