// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_delivery_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookDeliveryPage extends WebhookDeliveryPage {
  @override
  final BuiltList<WebhookDelivery> items;
  @override
  final String? nextCursor;

  factory _$WebhookDeliveryPage([
    void Function(WebhookDeliveryPageBuilder)? updates,
  ]) => (WebhookDeliveryPageBuilder()..update(updates))._build();

  _$WebhookDeliveryPage._({required this.items, this.nextCursor}) : super._();
  @override
  WebhookDeliveryPage rebuild(
    void Function(WebhookDeliveryPageBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookDeliveryPageBuilder toBuilder() =>
      WebhookDeliveryPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookDeliveryPage &&
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
    return (newBuiltValueToStringHelper(r'WebhookDeliveryPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class WebhookDeliveryPageBuilder
    implements Builder<WebhookDeliveryPage, WebhookDeliveryPageBuilder> {
  _$WebhookDeliveryPage? _$v;

  ListBuilder<WebhookDelivery>? _items;
  ListBuilder<WebhookDelivery> get items =>
      _$this._items ??= ListBuilder<WebhookDelivery>();
  set items(ListBuilder<WebhookDelivery>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  WebhookDeliveryPageBuilder() {
    WebhookDeliveryPage._defaults(this);
  }

  WebhookDeliveryPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookDeliveryPage other) {
    _$v = other as _$WebhookDeliveryPage;
  }

  @override
  void update(void Function(WebhookDeliveryPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookDeliveryPage build() => _build();

  _$WebhookDeliveryPage _build() {
    _$WebhookDeliveryPage _$result;
    try {
      _$result =
          _$v ??
          _$WebhookDeliveryPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookDeliveryPage',
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
