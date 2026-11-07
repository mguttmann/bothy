// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakePage extends RunWakePage {
  @override
  final BuiltList<RunWake> items;
  @override
  final String? nextCursor;

  factory _$RunWakePage([void Function(RunWakePageBuilder)? updates]) =>
      (RunWakePageBuilder()..update(updates))._build();

  _$RunWakePage._({required this.items, this.nextCursor}) : super._();
  @override
  RunWakePage rebuild(void Function(RunWakePageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakePageBuilder toBuilder() => RunWakePageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakePage &&
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
    return (newBuiltValueToStringHelper(r'RunWakePage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class RunWakePageBuilder implements Builder<RunWakePage, RunWakePageBuilder> {
  _$RunWakePage? _$v;

  ListBuilder<RunWake>? _items;
  ListBuilder<RunWake> get items => _$this._items ??= ListBuilder<RunWake>();
  set items(ListBuilder<RunWake>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  RunWakePageBuilder() {
    RunWakePage._defaults(this);
  }

  RunWakePageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakePage other) {
    _$v = other as _$RunWakePage;
  }

  @override
  void update(void Function(RunWakePageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakePage build() => _build();

  _$RunWakePage _build() {
    _$RunWakePage _$result;
    try {
      _$result =
          _$v ?? _$RunWakePage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunWakePage',
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
