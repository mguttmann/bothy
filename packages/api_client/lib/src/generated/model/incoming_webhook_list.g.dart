// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhookList extends IncomingWebhookList {
  @override
  final BuiltList<IncomingWebhook> items;

  factory _$IncomingWebhookList([
    void Function(IncomingWebhookListBuilder)? updates,
  ]) => (IncomingWebhookListBuilder()..update(updates))._build();

  _$IncomingWebhookList._({required this.items}) : super._();
  @override
  IncomingWebhookList rebuild(
    void Function(IncomingWebhookListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookListBuilder toBuilder() =>
      IncomingWebhookListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookList && items == other.items;
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
      r'IncomingWebhookList',
    )..add('items', items)).toString();
  }
}

class IncomingWebhookListBuilder
    implements Builder<IncomingWebhookList, IncomingWebhookListBuilder> {
  _$IncomingWebhookList? _$v;

  ListBuilder<IncomingWebhook>? _items;
  ListBuilder<IncomingWebhook> get items =>
      _$this._items ??= ListBuilder<IncomingWebhook>();
  set items(ListBuilder<IncomingWebhook>? items) => _$this._items = items;

  IncomingWebhookListBuilder() {
    IncomingWebhookList._defaults(this);
  }

  IncomingWebhookListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookList other) {
    _$v = other as _$IncomingWebhookList;
  }

  @override
  void update(void Function(IncomingWebhookListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookList build() => _build();

  _$IncomingWebhookList _build() {
    _$IncomingWebhookList _$result;
    try {
      _$result = _$v ?? _$IncomingWebhookList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IncomingWebhookList',
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
