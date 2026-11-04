// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_history_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunHistoryPage extends RunHistoryPage {
  @override
  final BuiltList<RunHistoryEntry> items;
  @override
  final String? nextCursor;

  factory _$RunHistoryPage([void Function(RunHistoryPageBuilder)? updates]) =>
      (RunHistoryPageBuilder()..update(updates))._build();

  _$RunHistoryPage._({required this.items, this.nextCursor}) : super._();
  @override
  RunHistoryPage rebuild(void Function(RunHistoryPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunHistoryPageBuilder toBuilder() => RunHistoryPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunHistoryPage &&
        items == other.items &&
        nextCursor == other.nextCursor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunHistoryPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class RunHistoryPageBuilder
    implements Builder<RunHistoryPage, RunHistoryPageBuilder> {
  _$RunHistoryPage? _$v;

  ListBuilder<RunHistoryEntry>? _items;
  ListBuilder<RunHistoryEntry> get items =>
      _$this._items ??= ListBuilder<RunHistoryEntry>();
  set items(ListBuilder<RunHistoryEntry>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  RunHistoryPageBuilder() {
    RunHistoryPage._defaults(this);
  }

  RunHistoryPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunHistoryPage other) {
    _$v = other as _$RunHistoryPage;
  }

  @override
  void update(void Function(RunHistoryPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunHistoryPage build() => _build();

  _$RunHistoryPage _build() {
    _$RunHistoryPage _$result;
    try {
      _$result =
          _$v ??
          _$RunHistoryPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunHistoryPage',
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
