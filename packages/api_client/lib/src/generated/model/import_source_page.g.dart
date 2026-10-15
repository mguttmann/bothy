// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportSourcePage extends ImportSourcePage {
  @override
  final BuiltList<ImportSource> items;
  @override
  final String? nextCursor;

  factory _$ImportSourcePage([
    void Function(ImportSourcePageBuilder)? updates,
  ]) => (ImportSourcePageBuilder()..update(updates))._build();

  _$ImportSourcePage._({required this.items, this.nextCursor}) : super._();
  @override
  ImportSourcePage rebuild(void Function(ImportSourcePageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportSourcePageBuilder toBuilder() =>
      ImportSourcePageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourcePage &&
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
    return (newBuiltValueToStringHelper(r'ImportSourcePage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ImportSourcePageBuilder
    implements Builder<ImportSourcePage, ImportSourcePageBuilder> {
  _$ImportSourcePage? _$v;

  ListBuilder<ImportSource>? _items;
  ListBuilder<ImportSource> get items =>
      _$this._items ??= ListBuilder<ImportSource>();
  set items(ListBuilder<ImportSource>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ImportSourcePageBuilder() {
    ImportSourcePage._defaults(this);
  }

  ImportSourcePageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourcePage other) {
    _$v = other as _$ImportSourcePage;
  }

  @override
  void update(void Function(ImportSourcePageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourcePage build() => _build();

  _$ImportSourcePage _build() {
    _$ImportSourcePage _$result;
    try {
      _$result =
          _$v ??
          _$ImportSourcePage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportSourcePage',
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
