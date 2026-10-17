// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadPage extends ThreadPage {
  @override
  final BuiltList<Thread> items;
  @override
  final String? nextCursor;

  factory _$ThreadPage([void Function(ThreadPageBuilder)? updates]) =>
      (ThreadPageBuilder()..update(updates))._build();

  _$ThreadPage._({required this.items, this.nextCursor}) : super._();
  @override
  ThreadPage rebuild(void Function(ThreadPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadPageBuilder toBuilder() => ThreadPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadPage &&
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
    return (newBuiltValueToStringHelper(r'ThreadPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ThreadPageBuilder implements Builder<ThreadPage, ThreadPageBuilder> {
  _$ThreadPage? _$v;

  ListBuilder<Thread>? _items;
  ListBuilder<Thread> get items => _$this._items ??= ListBuilder<Thread>();
  set items(ListBuilder<Thread>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ThreadPageBuilder() {
    ThreadPage._defaults(this);
  }

  ThreadPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadPage other) {
    _$v = other as _$ThreadPage;
  }

  @override
  void update(void Function(ThreadPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadPage build() => _build();

  _$ThreadPage _build() {
    _$ThreadPage _$result;
    try {
      _$result =
          _$v ?? _$ThreadPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadPage',
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
