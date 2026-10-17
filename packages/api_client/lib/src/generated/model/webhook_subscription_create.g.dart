// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_subscription_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookSubscriptionCreate extends WebhookSubscriptionCreate {
  @override
  final String url;
  @override
  final BuiltList<String> eventTypes;
  @override
  final bool? enableHmac;

  factory _$WebhookSubscriptionCreate([
    void Function(WebhookSubscriptionCreateBuilder)? updates,
  ]) => (WebhookSubscriptionCreateBuilder()..update(updates))._build();

  _$WebhookSubscriptionCreate._({
    required this.url,
    required this.eventTypes,
    this.enableHmac,
  }) : super._();
  @override
  WebhookSubscriptionCreate rebuild(
    void Function(WebhookSubscriptionCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookSubscriptionCreateBuilder toBuilder() =>
      WebhookSubscriptionCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookSubscriptionCreate &&
        url == other.url &&
        eventTypes == other.eventTypes &&
        enableHmac == other.enableHmac;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, eventTypes.hashCode);
    _$hash = $jc(_$hash, enableHmac.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookSubscriptionCreate')
          ..add('url', url)
          ..add('eventTypes', eventTypes)
          ..add('enableHmac', enableHmac))
        .toString();
  }
}

class WebhookSubscriptionCreateBuilder
    implements
        Builder<WebhookSubscriptionCreate, WebhookSubscriptionCreateBuilder> {
  _$WebhookSubscriptionCreate? _$v;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<String>? _eventTypes;
  ListBuilder<String> get eventTypes =>
      _$this._eventTypes ??= ListBuilder<String>();
  set eventTypes(ListBuilder<String>? eventTypes) =>
      _$this._eventTypes = eventTypes;

  bool? _enableHmac;
  bool? get enableHmac => _$this._enableHmac;
  set enableHmac(bool? enableHmac) => _$this._enableHmac = enableHmac;

  WebhookSubscriptionCreateBuilder() {
    WebhookSubscriptionCreate._defaults(this);
  }

  WebhookSubscriptionCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _url = $v.url;
      _eventTypes = $v.eventTypes.toBuilder();
      _enableHmac = $v.enableHmac;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookSubscriptionCreate other) {
    _$v = other as _$WebhookSubscriptionCreate;
  }

  @override
  void update(void Function(WebhookSubscriptionCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookSubscriptionCreate build() => _build();

  _$WebhookSubscriptionCreate _build() {
    _$WebhookSubscriptionCreate _$result;
    try {
      _$result =
          _$v ??
          _$WebhookSubscriptionCreate._(
            url: BuiltValueNullFieldError.checkNotNull(
              url,
              r'WebhookSubscriptionCreate',
              'url',
            ),
            eventTypes: eventTypes.build(),
            enableHmac: enableHmac,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'eventTypes';
        eventTypes.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookSubscriptionCreate',
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
