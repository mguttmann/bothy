// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessagePage extends MessagePage {
  @override
  final BuiltList<Message> items;
  @override
  final bool hasMoreBefore;
  @override
  final bool hasMoreAfter;

  factory _$MessagePage([void Function(MessagePageBuilder)? updates]) =>
      (MessagePageBuilder()..update(updates))._build();

  _$MessagePage._({
    required this.items,
    required this.hasMoreBefore,
    required this.hasMoreAfter,
  }) : super._();
  @override
  MessagePage rebuild(void Function(MessagePageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessagePageBuilder toBuilder() => MessagePageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessagePage &&
        items == other.items &&
        hasMoreBefore == other.hasMoreBefore &&
        hasMoreAfter == other.hasMoreAfter;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, hasMoreBefore.hashCode);
    _$hash = $jc(_$hash, hasMoreAfter.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MessagePage')
          ..add('items', items)
          ..add('hasMoreBefore', hasMoreBefore)
          ..add('hasMoreAfter', hasMoreAfter))
        .toString();
  }
}

class MessagePageBuilder implements Builder<MessagePage, MessagePageBuilder> {
  _$MessagePage? _$v;

  ListBuilder<Message>? _items;
  ListBuilder<Message> get items => _$this._items ??= ListBuilder<Message>();
  set items(ListBuilder<Message>? items) => _$this._items = items;

  bool? _hasMoreBefore;
  bool? get hasMoreBefore => _$this._hasMoreBefore;
  set hasMoreBefore(bool? hasMoreBefore) =>
      _$this._hasMoreBefore = hasMoreBefore;

  bool? _hasMoreAfter;
  bool? get hasMoreAfter => _$this._hasMoreAfter;
  set hasMoreAfter(bool? hasMoreAfter) => _$this._hasMoreAfter = hasMoreAfter;

  MessagePageBuilder() {
    MessagePage._defaults(this);
  }

  MessagePageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _hasMoreBefore = $v.hasMoreBefore;
      _hasMoreAfter = $v.hasMoreAfter;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessagePage other) {
    _$v = other as _$MessagePage;
  }

  @override
  void update(void Function(MessagePageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessagePage build() => _build();

  _$MessagePage _build() {
    _$MessagePage _$result;
    try {
      _$result =
          _$v ??
          _$MessagePage._(
            items: items.build(),
            hasMoreBefore: BuiltValueNullFieldError.checkNotNull(
              hasMoreBefore,
              r'MessagePage',
              'hasMoreBefore',
            ),
            hasMoreAfter: BuiltValueNullFieldError.checkNotNull(
              hasMoreAfter,
              r'MessagePage',
              'hasMoreAfter',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MessagePage',
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
