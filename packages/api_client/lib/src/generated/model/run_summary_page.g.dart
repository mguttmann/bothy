// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_summary_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunSummaryPage extends RunSummaryPage {
  @override
  final BuiltList<RunSummary> items;
  @override
  final String? nextCursor;

  factory _$RunSummaryPage([void Function(RunSummaryPageBuilder)? updates]) =>
      (RunSummaryPageBuilder()..update(updates))._build();

  _$RunSummaryPage._({required this.items, this.nextCursor}) : super._();
  @override
  RunSummaryPage rebuild(void Function(RunSummaryPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunSummaryPageBuilder toBuilder() => RunSummaryPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunSummaryPage &&
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
    return (newBuiltValueToStringHelper(r'RunSummaryPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class RunSummaryPageBuilder
    implements Builder<RunSummaryPage, RunSummaryPageBuilder> {
  _$RunSummaryPage? _$v;

  ListBuilder<RunSummary>? _items;
  ListBuilder<RunSummary> get items =>
      _$this._items ??= ListBuilder<RunSummary>();
  set items(ListBuilder<RunSummary>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  RunSummaryPageBuilder() {
    RunSummaryPage._defaults(this);
  }

  RunSummaryPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunSummaryPage other) {
    _$v = other as _$RunSummaryPage;
  }

  @override
  void update(void Function(RunSummaryPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunSummaryPage build() => _build();

  _$RunSummaryPage _build() {
    _$RunSummaryPage _$result;
    try {
      _$result =
          _$v ??
          _$RunSummaryPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunSummaryPage',
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
