// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_subscription_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookSubscriptionCreated extends WebhookSubscriptionCreated {
  @override
  final WebhookSubscription subscription;
  @override
  final String? hmacSecret;

  factory _$WebhookSubscriptionCreated([
    void Function(WebhookSubscriptionCreatedBuilder)? updates,
  ]) => (WebhookSubscriptionCreatedBuilder()..update(updates))._build();

  _$WebhookSubscriptionCreated._({required this.subscription, this.hmacSecret})
    : super._();
  @override
  WebhookSubscriptionCreated rebuild(
    void Function(WebhookSubscriptionCreatedBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookSubscriptionCreatedBuilder toBuilder() =>
      WebhookSubscriptionCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookSubscriptionCreated &&
        subscription == other.subscription &&
        hmacSecret == other.hmacSecret;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, subscription.hashCode);
    _$hash = $jc(_$hash, hmacSecret.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookSubscriptionCreated')
          ..add('subscription', subscription)
          ..add('hmacSecret', hmacSecret))
        .toString();
  }
}

class WebhookSubscriptionCreatedBuilder
    implements
        Builder<WebhookSubscriptionCreated, WebhookSubscriptionCreatedBuilder> {
  _$WebhookSubscriptionCreated? _$v;

  WebhookSubscriptionBuilder? _subscription;
  WebhookSubscriptionBuilder get subscription =>
      _$this._subscription ??= WebhookSubscriptionBuilder();
  set subscription(WebhookSubscriptionBuilder? subscription) =>
      _$this._subscription = subscription;

  String? _hmacSecret;
  String? get hmacSecret => _$this._hmacSecret;
  set hmacSecret(String? hmacSecret) => _$this._hmacSecret = hmacSecret;

  WebhookSubscriptionCreatedBuilder() {
    WebhookSubscriptionCreated._defaults(this);
  }

  WebhookSubscriptionCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _subscription = $v.subscription.toBuilder();
      _hmacSecret = $v.hmacSecret;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookSubscriptionCreated other) {
    _$v = other as _$WebhookSubscriptionCreated;
  }

  @override
  void update(void Function(WebhookSubscriptionCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookSubscriptionCreated build() => _build();

  _$WebhookSubscriptionCreated _build() {
    _$WebhookSubscriptionCreated _$result;
    try {
      _$result =
          _$v ??
          _$WebhookSubscriptionCreated._(
            subscription: subscription.build(),
            hmacSecret: hmacSecret,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'subscription';
        subscription.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookSubscriptionCreated',
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
