// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_subscription_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookSubscriptionList extends WebhookSubscriptionList {
  @override
  final BuiltList<WebhookSubscription> items;

  factory _$WebhookSubscriptionList([
    void Function(WebhookSubscriptionListBuilder)? updates,
  ]) => (WebhookSubscriptionListBuilder()..update(updates))._build();

  _$WebhookSubscriptionList._({required this.items}) : super._();
  @override
  WebhookSubscriptionList rebuild(
    void Function(WebhookSubscriptionListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookSubscriptionListBuilder toBuilder() =>
      WebhookSubscriptionListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookSubscriptionList && items == other.items;
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
      r'WebhookSubscriptionList',
    )..add('items', items)).toString();
  }
}

class WebhookSubscriptionListBuilder
    implements
        Builder<WebhookSubscriptionList, WebhookSubscriptionListBuilder> {
  _$WebhookSubscriptionList? _$v;

  ListBuilder<WebhookSubscription>? _items;
  ListBuilder<WebhookSubscription> get items =>
      _$this._items ??= ListBuilder<WebhookSubscription>();
  set items(ListBuilder<WebhookSubscription>? items) => _$this._items = items;

  WebhookSubscriptionListBuilder() {
    WebhookSubscriptionList._defaults(this);
  }

  WebhookSubscriptionListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookSubscriptionList other) {
    _$v = other as _$WebhookSubscriptionList;
  }

  @override
  void update(void Function(WebhookSubscriptionListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookSubscriptionList build() => _build();

  _$WebhookSubscriptionList _build() {
    _$WebhookSubscriptionList _$result;
    try {
      _$result = _$v ?? _$WebhookSubscriptionList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookSubscriptionList',
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
