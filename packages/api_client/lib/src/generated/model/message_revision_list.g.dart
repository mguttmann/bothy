// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_revision_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessageRevisionList extends MessageRevisionList {
  @override
  final BuiltList<MessageRevision> items;

  factory _$MessageRevisionList([
    void Function(MessageRevisionListBuilder)? updates,
  ]) => (MessageRevisionListBuilder()..update(updates))._build();

  _$MessageRevisionList._({required this.items}) : super._();
  @override
  MessageRevisionList rebuild(
    void Function(MessageRevisionListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  MessageRevisionListBuilder toBuilder() =>
      MessageRevisionListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageRevisionList && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'MessageRevisionList',
    )..add('items', items)).toString();
  }
}

class MessageRevisionListBuilder
    implements Builder<MessageRevisionList, MessageRevisionListBuilder> {
  _$MessageRevisionList? _$v;

  ListBuilder<MessageRevision>? _items;
  ListBuilder<MessageRevision> get items =>
      _$this._items ??= ListBuilder<MessageRevision>();
  set items(ListBuilder<MessageRevision>? items) => _$this._items = items;

  MessageRevisionListBuilder() {
    MessageRevisionList._defaults(this);
  }

  MessageRevisionListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessageRevisionList other) {
    _$v = other as _$MessageRevisionList;
  }

  @override
  void update(void Function(MessageRevisionListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageRevisionList build() => _build();

  _$MessageRevisionList _build() {
    _$MessageRevisionList _$result;
    try {
      _$result = _$v ?? _$MessageRevisionList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MessageRevisionList',
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
