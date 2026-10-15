// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SessionPage extends SessionPage {
  @override
  final BuiltList<Session> items;
  @override
  final String? nextCursor;

  factory _$SessionPage([void Function(SessionPageBuilder)? updates]) =>
      (SessionPageBuilder()..update(updates))._build();

  _$SessionPage._({required this.items, this.nextCursor}) : super._();
  @override
  SessionPage rebuild(void Function(SessionPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SessionPageBuilder toBuilder() => SessionPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SessionPage &&
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
    return (newBuiltValueToStringHelper(r'SessionPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class SessionPageBuilder implements Builder<SessionPage, SessionPageBuilder> {
  _$SessionPage? _$v;

  ListBuilder<Session>? _items;
  ListBuilder<Session> get items => _$this._items ??= ListBuilder<Session>();
  set items(ListBuilder<Session>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  SessionPageBuilder() {
    SessionPage._defaults(this);
  }

  SessionPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SessionPage other) {
    _$v = other as _$SessionPage;
  }

  @override
  void update(void Function(SessionPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SessionPage build() => _build();

  _$SessionPage _build() {
    _$SessionPage _$result;
    try {
      _$result =
          _$v ?? _$SessionPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SessionPage',
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
