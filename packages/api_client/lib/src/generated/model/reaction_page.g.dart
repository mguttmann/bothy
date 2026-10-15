// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reaction_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReactionPage extends ReactionPage {
  @override
  final BuiltList<Reaction> items;
  @override
  final String? nextCursor;

  factory _$ReactionPage([void Function(ReactionPageBuilder)? updates]) =>
      (ReactionPageBuilder()..update(updates))._build();

  _$ReactionPage._({required this.items, this.nextCursor}) : super._();
  @override
  ReactionPage rebuild(void Function(ReactionPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReactionPageBuilder toBuilder() => ReactionPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReactionPage &&
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
    return (newBuiltValueToStringHelper(r'ReactionPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ReactionPageBuilder
    implements Builder<ReactionPage, ReactionPageBuilder> {
  _$ReactionPage? _$v;

  ListBuilder<Reaction>? _items;
  ListBuilder<Reaction> get items => _$this._items ??= ListBuilder<Reaction>();
  set items(ListBuilder<Reaction>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ReactionPageBuilder() {
    ReactionPage._defaults(this);
  }

  ReactionPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReactionPage other) {
    _$v = other as _$ReactionPage;
  }

  @override
  void update(void Function(ReactionPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReactionPage build() => _build();

  _$ReactionPage _build() {
    _$ReactionPage _$result;
    try {
      _$result =
          _$v ?? _$ReactionPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ReactionPage',
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
