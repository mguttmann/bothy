// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakeList extends RunWakeList {
  @override
  final BuiltList<RunWake> items;

  factory _$RunWakeList([void Function(RunWakeListBuilder)? updates]) =>
      (RunWakeListBuilder()..update(updates))._build();

  _$RunWakeList._({required this.items}) : super._();
  @override
  RunWakeList rebuild(void Function(RunWakeListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeListBuilder toBuilder() => RunWakeListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeList && items == other.items;
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
      r'RunWakeList',
    )..add('items', items)).toString();
  }
}

class RunWakeListBuilder implements Builder<RunWakeList, RunWakeListBuilder> {
  _$RunWakeList? _$v;

  ListBuilder<RunWake>? _items;
  ListBuilder<RunWake> get items => _$this._items ??= ListBuilder<RunWake>();
  set items(ListBuilder<RunWake>? items) => _$this._items = items;

  RunWakeListBuilder() {
    RunWakeList._defaults(this);
  }

  RunWakeListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeList other) {
    _$v = other as _$RunWakeList;
  }

  @override
  void update(void Function(RunWakeListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeList build() => _build();

  _$RunWakeList _build() {
    _$RunWakeList _$result;
    try {
      _$result = _$v ?? _$RunWakeList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunWakeList',
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
