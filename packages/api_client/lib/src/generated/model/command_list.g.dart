// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandList extends CommandList {
  @override
  final BuiltList<Command> items;

  factory _$CommandList([void Function(CommandListBuilder)? updates]) =>
      (CommandListBuilder()..update(updates))._build();

  _$CommandList._({required this.items}) : super._();
  @override
  CommandList rebuild(void Function(CommandListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandListBuilder toBuilder() => CommandListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandList && items == other.items;
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
      r'CommandList',
    )..add('items', items)).toString();
  }
}

class CommandListBuilder implements Builder<CommandList, CommandListBuilder> {
  _$CommandList? _$v;

  ListBuilder<Command>? _items;
  ListBuilder<Command> get items => _$this._items ??= ListBuilder<Command>();
  set items(ListBuilder<Command>? items) => _$this._items = items;

  CommandListBuilder() {
    CommandList._defaults(this);
  }

  CommandListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandList other) {
    _$v = other as _$CommandList;
  }

  @override
  void update(void Function(CommandListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandList build() => _build();

  _$CommandList _build() {
    _$CommandList _$result;
    try {
      _$result = _$v ?? _$CommandList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandList',
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
