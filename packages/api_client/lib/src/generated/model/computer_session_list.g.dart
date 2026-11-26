// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_session_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerSessionList extends ComputerSessionList {
  @override
  final BuiltList<ComputerSession> items;

  factory _$ComputerSessionList([
    void Function(ComputerSessionListBuilder)? updates,
  ]) => (ComputerSessionListBuilder()..update(updates))._build();

  _$ComputerSessionList._({required this.items}) : super._();
  @override
  ComputerSessionList rebuild(
    void Function(ComputerSessionListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerSessionListBuilder toBuilder() =>
      ComputerSessionListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerSessionList && items == other.items;
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
      r'ComputerSessionList',
    )..add('items', items)).toString();
  }
}

class ComputerSessionListBuilder
    implements Builder<ComputerSessionList, ComputerSessionListBuilder> {
  _$ComputerSessionList? _$v;

  ListBuilder<ComputerSession>? _items;
  ListBuilder<ComputerSession> get items =>
      _$this._items ??= ListBuilder<ComputerSession>();
  set items(ListBuilder<ComputerSession>? items) => _$this._items = items;

  ComputerSessionListBuilder() {
    ComputerSessionList._defaults(this);
  }

  ComputerSessionListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerSessionList other) {
    _$v = other as _$ComputerSessionList;
  }

  @override
  void update(void Function(ComputerSessionListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerSessionList build() => _build();

  _$ComputerSessionList _build() {
    _$ComputerSessionList _$result;
    try {
      _$result = _$v ?? _$ComputerSessionList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ComputerSessionList',
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
