// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'mention_item_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MentionItemPage extends MentionItemPage {
  @override
  final BuiltList<MentionItem> items;
  @override
  final String? nextCursor;

  factory _$MentionItemPage([void Function(MentionItemPageBuilder)? updates]) =>
      (MentionItemPageBuilder()..update(updates))._build();

  _$MentionItemPage._({required this.items, this.nextCursor}) : super._();
  @override
  MentionItemPage rebuild(void Function(MentionItemPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MentionItemPageBuilder toBuilder() => MentionItemPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MentionItemPage &&
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
    return (newBuiltValueToStringHelper(r'MentionItemPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class MentionItemPageBuilder
    implements Builder<MentionItemPage, MentionItemPageBuilder> {
  _$MentionItemPage? _$v;

  ListBuilder<MentionItem>? _items;
  ListBuilder<MentionItem> get items =>
      _$this._items ??= ListBuilder<MentionItem>();
  set items(ListBuilder<MentionItem>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  MentionItemPageBuilder() {
    MentionItemPage._defaults(this);
  }

  MentionItemPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MentionItemPage other) {
    _$v = other as _$MentionItemPage;
  }

  @override
  void update(void Function(MentionItemPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MentionItemPage build() => _build();

  _$MentionItemPage _build() {
    _$MentionItemPage _$result;
    try {
      _$result =
          _$v ??
          _$MentionItemPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MentionItemPage',
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
