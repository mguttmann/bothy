// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GrantPage extends GrantPage {
  @override
  final BuiltList<Grant> items;
  @override
  final String? nextCursor;

  factory _$GrantPage([void Function(GrantPageBuilder)? updates]) =>
      (GrantPageBuilder()..update(updates))._build();

  _$GrantPage._({required this.items, this.nextCursor}) : super._();
  @override
  GrantPage rebuild(void Function(GrantPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GrantPageBuilder toBuilder() => GrantPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GrantPage &&
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
    return (newBuiltValueToStringHelper(r'GrantPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class GrantPageBuilder implements Builder<GrantPage, GrantPageBuilder> {
  _$GrantPage? _$v;

  ListBuilder<Grant>? _items;
  ListBuilder<Grant> get items => _$this._items ??= ListBuilder<Grant>();
  set items(ListBuilder<Grant>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  GrantPageBuilder() {
    GrantPage._defaults(this);
  }

  GrantPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GrantPage other) {
    _$v = other as _$GrantPage;
  }

  @override
  void update(void Function(GrantPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GrantPage build() => _build();

  _$GrantPage _build() {
    _$GrantPage _$result;
    try {
      _$result =
          _$v ?? _$GrantPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GrantPage',
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
