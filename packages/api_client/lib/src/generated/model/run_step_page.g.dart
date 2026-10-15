// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_step_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunStepPage extends RunStepPage {
  @override
  final BuiltList<RunStep> items;
  @override
  final String? nextCursor;

  factory _$RunStepPage([void Function(RunStepPageBuilder)? updates]) =>
      (RunStepPageBuilder()..update(updates))._build();

  _$RunStepPage._({required this.items, this.nextCursor}) : super._();
  @override
  RunStepPage rebuild(void Function(RunStepPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunStepPageBuilder toBuilder() => RunStepPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunStepPage &&
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
    return (newBuiltValueToStringHelper(r'RunStepPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class RunStepPageBuilder implements Builder<RunStepPage, RunStepPageBuilder> {
  _$RunStepPage? _$v;

  ListBuilder<RunStep>? _items;
  ListBuilder<RunStep> get items => _$this._items ??= ListBuilder<RunStep>();
  set items(ListBuilder<RunStep>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  RunStepPageBuilder() {
    RunStepPage._defaults(this);
  }

  RunStepPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunStepPage other) {
    _$v = other as _$RunStepPage;
  }

  @override
  void update(void Function(RunStepPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunStepPage build() => _build();

  _$RunStepPage _build() {
    _$RunStepPage _$result;
    try {
      _$result =
          _$v ?? _$RunStepPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunStepPage',
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
