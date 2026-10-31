// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_state_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarStateList extends SidebarStateList {
  @override
  final BuiltList<SidebarState> items;
  @override
  final BuiltList<SidebarChannelState>? channels;

  factory _$SidebarStateList([
    void Function(SidebarStateListBuilder)? updates,
  ]) => (SidebarStateListBuilder()..update(updates))._build();

  _$SidebarStateList._({required this.items, this.channels}) : super._();
  @override
  SidebarStateList rebuild(void Function(SidebarStateListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SidebarStateListBuilder toBuilder() =>
      SidebarStateListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarStateList &&
        items == other.items &&
        channels == other.channels;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SidebarStateList')
          ..add('items', items)
          ..add('channels', channels))
        .toString();
  }
}

class SidebarStateListBuilder
    implements Builder<SidebarStateList, SidebarStateListBuilder> {
  _$SidebarStateList? _$v;

  ListBuilder<SidebarState>? _items;
  ListBuilder<SidebarState> get items =>
      _$this._items ??= ListBuilder<SidebarState>();
  set items(ListBuilder<SidebarState>? items) => _$this._items = items;

  ListBuilder<SidebarChannelState>? _channels;
  ListBuilder<SidebarChannelState> get channels =>
      _$this._channels ??= ListBuilder<SidebarChannelState>();
  set channels(ListBuilder<SidebarChannelState>? channels) =>
      _$this._channels = channels;

  SidebarStateListBuilder() {
    SidebarStateList._defaults(this);
  }

  SidebarStateListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _channels = $v.channels?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarStateList other) {
    _$v = other as _$SidebarStateList;
  }

  @override
  void update(void Function(SidebarStateListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarStateList build() => _build();

  _$SidebarStateList _build() {
    _$SidebarStateList _$result;
    try {
      _$result =
          _$v ??
          _$SidebarStateList._(
            items: items.build(),
            channels: _channels?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
        _$failedField = 'channels';
        _channels?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarStateList',
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
