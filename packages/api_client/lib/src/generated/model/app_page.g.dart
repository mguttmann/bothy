// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppPage extends AppPage {
  @override
  final BuiltList<App> items;
  @override
  final String? nextCursor;

  factory _$AppPage([void Function(AppPageBuilder)? updates]) =>
      (AppPageBuilder()..update(updates))._build();

  _$AppPage._({required this.items, this.nextCursor}) : super._();
  @override
  AppPage rebuild(void Function(AppPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppPageBuilder toBuilder() => AppPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppPage &&
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
    return (newBuiltValueToStringHelper(r'AppPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class AppPageBuilder implements Builder<AppPage, AppPageBuilder> {
  _$AppPage? _$v;

  ListBuilder<App>? _items;
  ListBuilder<App> get items => _$this._items ??= ListBuilder<App>();
  set items(ListBuilder<App>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  AppPageBuilder() {
    AppPage._defaults(this);
  }

  AppPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppPage other) {
    _$v = other as _$AppPage;
  }

  @override
  void update(void Function(AppPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppPage build() => _build();

  _$AppPage _build() {
    _$AppPage _$result;
    try {
      _$result =
          _$v ?? _$AppPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppPage',
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
