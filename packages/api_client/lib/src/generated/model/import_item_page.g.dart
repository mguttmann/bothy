// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_item_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportItemPage extends ImportItemPage {
  @override
  final BuiltList<ImportItem> items;
  @override
  final String? nextCursor;

  factory _$ImportItemPage([void Function(ImportItemPageBuilder)? updates]) =>
      (ImportItemPageBuilder()..update(updates))._build();

  _$ImportItemPage._({required this.items, this.nextCursor}) : super._();
  @override
  ImportItemPage rebuild(void Function(ImportItemPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportItemPageBuilder toBuilder() => ImportItemPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportItemPage &&
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
    return (newBuiltValueToStringHelper(r'ImportItemPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ImportItemPageBuilder
    implements Builder<ImportItemPage, ImportItemPageBuilder> {
  _$ImportItemPage? _$v;

  ListBuilder<ImportItem>? _items;
  ListBuilder<ImportItem> get items =>
      _$this._items ??= ListBuilder<ImportItem>();
  set items(ListBuilder<ImportItem>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ImportItemPageBuilder() {
    ImportItemPage._defaults(this);
  }

  ImportItemPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportItemPage other) {
    _$v = other as _$ImportItemPage;
  }

  @override
  void update(void Function(ImportItemPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportItemPage build() => _build();

  _$ImportItemPage _build() {
    _$ImportItemPage _$result;
    try {
      _$result =
          _$v ??
          _$ImportItemPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportItemPage',
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
