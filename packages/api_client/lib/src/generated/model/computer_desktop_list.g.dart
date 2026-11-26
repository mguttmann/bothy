// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_desktop_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerDesktopList extends ComputerDesktopList {
  @override
  final BuiltList<ComputerDesktop> items;

  factory _$ComputerDesktopList([
    void Function(ComputerDesktopListBuilder)? updates,
  ]) => (ComputerDesktopListBuilder()..update(updates))._build();

  _$ComputerDesktopList._({required this.items}) : super._();
  @override
  ComputerDesktopList rebuild(
    void Function(ComputerDesktopListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerDesktopListBuilder toBuilder() =>
      ComputerDesktopListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerDesktopList && items == other.items;
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
      r'ComputerDesktopList',
    )..add('items', items)).toString();
  }
}

class ComputerDesktopListBuilder
    implements Builder<ComputerDesktopList, ComputerDesktopListBuilder> {
  _$ComputerDesktopList? _$v;

  ListBuilder<ComputerDesktop>? _items;
  ListBuilder<ComputerDesktop> get items =>
      _$this._items ??= ListBuilder<ComputerDesktop>();
  set items(ListBuilder<ComputerDesktop>? items) => _$this._items = items;

  ComputerDesktopListBuilder() {
    ComputerDesktopList._defaults(this);
  }

  ComputerDesktopListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerDesktopList other) {
    _$v = other as _$ComputerDesktopList;
  }

  @override
  void update(void Function(ComputerDesktopListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerDesktopList build() => _build();

  _$ComputerDesktopList _build() {
    _$ComputerDesktopList _$result;
    try {
      _$result = _$v ?? _$ComputerDesktopList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ComputerDesktopList',
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
