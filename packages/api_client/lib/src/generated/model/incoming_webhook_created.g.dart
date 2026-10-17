// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhookCreated extends IncomingWebhookCreated {
  @override
  final IncomingWebhook webhook;
  @override
  final String url;

  factory _$IncomingWebhookCreated([
    void Function(IncomingWebhookCreatedBuilder)? updates,
  ]) => (IncomingWebhookCreatedBuilder()..update(updates))._build();

  _$IncomingWebhookCreated._({required this.webhook, required this.url})
    : super._();
  @override
  IncomingWebhookCreated rebuild(
    void Function(IncomingWebhookCreatedBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookCreatedBuilder toBuilder() =>
      IncomingWebhookCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookCreated &&
        webhook == other.webhook &&
        url == other.url;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, webhook.hashCode);
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhookCreated')
          ..add('webhook', webhook)
          ..add('url', url))
        .toString();
  }
}

class IncomingWebhookCreatedBuilder
    implements Builder<IncomingWebhookCreated, IncomingWebhookCreatedBuilder> {
  _$IncomingWebhookCreated? _$v;

  IncomingWebhookBuilder? _webhook;
  IncomingWebhookBuilder get webhook =>
      _$this._webhook ??= IncomingWebhookBuilder();
  set webhook(IncomingWebhookBuilder? webhook) => _$this._webhook = webhook;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  IncomingWebhookCreatedBuilder() {
    IncomingWebhookCreated._defaults(this);
  }

  IncomingWebhookCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _webhook = $v.webhook.toBuilder();
      _url = $v.url;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookCreated other) {
    _$v = other as _$IncomingWebhookCreated;
  }

  @override
  void update(void Function(IncomingWebhookCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookCreated build() => _build();

  _$IncomingWebhookCreated _build() {
    _$IncomingWebhookCreated _$result;
    try {
      _$result =
          _$v ??
          _$IncomingWebhookCreated._(
            webhook: webhook.build(),
            url: BuiltValueNullFieldError.checkNotNull(
              url,
              r'IncomingWebhookCreated',
              'url',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'webhook';
        webhook.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IncomingWebhookCreated',
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
