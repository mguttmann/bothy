// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalSessionPage extends TerminalSessionPage {
  @override
  final BuiltList<TerminalSession> items;
  @override
  final String? nextCursor;

  factory _$TerminalSessionPage([
    void Function(TerminalSessionPageBuilder)? updates,
  ]) => (TerminalSessionPageBuilder()..update(updates))._build();

  _$TerminalSessionPage._({required this.items, this.nextCursor}) : super._();
  @override
  TerminalSessionPage rebuild(
    void Function(TerminalSessionPageBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalSessionPageBuilder toBuilder() =>
      TerminalSessionPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalSessionPage &&
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
    return (newBuiltValueToStringHelper(r'TerminalSessionPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class TerminalSessionPageBuilder
    implements Builder<TerminalSessionPage, TerminalSessionPageBuilder> {
  _$TerminalSessionPage? _$v;

  ListBuilder<TerminalSession>? _items;
  ListBuilder<TerminalSession> get items =>
      _$this._items ??= ListBuilder<TerminalSession>();
  set items(ListBuilder<TerminalSession>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  TerminalSessionPageBuilder() {
    TerminalSessionPage._defaults(this);
  }

  TerminalSessionPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalSessionPage other) {
    _$v = other as _$TerminalSessionPage;
  }

  @override
  void update(void Function(TerminalSessionPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalSessionPage build() => _build();

  _$TerminalSessionPage _build() {
    _$TerminalSessionPage _$result;
    try {
      _$result =
          _$v ??
          _$TerminalSessionPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TerminalSessionPage',
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
