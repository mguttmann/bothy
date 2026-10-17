// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pin_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PinList extends PinList {
  @override
  final BuiltList<Pin> items;

  factory _$PinList([void Function(PinListBuilder)? updates]) =>
      (PinListBuilder()..update(updates))._build();

  _$PinList._({required this.items}) : super._();
  @override
  PinList rebuild(void Function(PinListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PinListBuilder toBuilder() => PinListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PinList && items == other.items;
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
      r'PinList',
    )..add('items', items)).toString();
  }
}

class PinListBuilder implements Builder<PinList, PinListBuilder> {
  _$PinList? _$v;

  ListBuilder<Pin>? _items;
  ListBuilder<Pin> get items => _$this._items ??= ListBuilder<Pin>();
  set items(ListBuilder<Pin>? items) => _$this._items = items;

  PinListBuilder() {
    PinList._defaults(this);
  }

  PinListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PinList other) {
    _$v = other as _$PinList;
  }

  @override
  void update(void Function(PinListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PinList build() => _build();

  _$PinList _build() {
    _$PinList _$result;
    try {
      _$result = _$v ?? _$PinList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PinList',
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
