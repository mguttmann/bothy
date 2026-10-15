// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_subscription_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const WebhookSubscriptionUpdateStateEnum
_$webhookSubscriptionUpdateStateEnum_active =
    const WebhookSubscriptionUpdateStateEnum._('active');
const WebhookSubscriptionUpdateStateEnum
_$webhookSubscriptionUpdateStateEnum_paused =
    const WebhookSubscriptionUpdateStateEnum._('paused');
const WebhookSubscriptionUpdateStateEnum
_$webhookSubscriptionUpdateStateEnum_unknownDefaultOpenApi =
    const WebhookSubscriptionUpdateStateEnum._('unknownDefaultOpenApi');

WebhookSubscriptionUpdateStateEnum _$webhookSubscriptionUpdateStateEnumValueOf(
  String name,
) {
  switch (name) {
    case 'active':
      return _$webhookSubscriptionUpdateStateEnum_active;
    case 'paused':
      return _$webhookSubscriptionUpdateStateEnum_paused;
    case 'unknownDefaultOpenApi':
      return _$webhookSubscriptionUpdateStateEnum_unknownDefaultOpenApi;
    default:
      return _$webhookSubscriptionUpdateStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<WebhookSubscriptionUpdateStateEnum>
_$webhookSubscriptionUpdateStateEnumValues =
    BuiltSet<WebhookSubscriptionUpdateStateEnum>(
      const <WebhookSubscriptionUpdateStateEnum>[
        _$webhookSubscriptionUpdateStateEnum_active,
        _$webhookSubscriptionUpdateStateEnum_paused,
        _$webhookSubscriptionUpdateStateEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<WebhookSubscriptionUpdateStateEnum>
_$webhookSubscriptionUpdateStateEnumSerializer =
    _$WebhookSubscriptionUpdateStateEnumSerializer();

class _$WebhookSubscriptionUpdateStateEnumSerializer
    implements PrimitiveSerializer<WebhookSubscriptionUpdateStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'paused': 'paused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'paused': 'paused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[WebhookSubscriptionUpdateStateEnum];
  @override
  final String wireName = 'WebhookSubscriptionUpdateStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionUpdateStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WebhookSubscriptionUpdateStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WebhookSubscriptionUpdateStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$WebhookSubscriptionUpdate extends WebhookSubscriptionUpdate {
  @override
  final String? url;
  @override
  final BuiltList<String>? eventTypes;
  @override
  final WebhookSubscriptionUpdateStateEnum? state;
  @override
  final bool? rotateHmacSecret;

  factory _$WebhookSubscriptionUpdate([
    void Function(WebhookSubscriptionUpdateBuilder)? updates,
  ]) => (WebhookSubscriptionUpdateBuilder()..update(updates))._build();

  _$WebhookSubscriptionUpdate._({
    this.url,
    this.eventTypes,
    this.state,
    this.rotateHmacSecret,
  }) : super._();
  @override
  WebhookSubscriptionUpdate rebuild(
    void Function(WebhookSubscriptionUpdateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookSubscriptionUpdateBuilder toBuilder() =>
      WebhookSubscriptionUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookSubscriptionUpdate &&
        url == other.url &&
        eventTypes == other.eventTypes &&
        state == other.state &&
        rotateHmacSecret == other.rotateHmacSecret;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, eventTypes.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, rotateHmacSecret.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookSubscriptionUpdate')
          ..add('url', url)
          ..add('eventTypes', eventTypes)
          ..add('state', state)
          ..add('rotateHmacSecret', rotateHmacSecret))
        .toString();
  }
}

class WebhookSubscriptionUpdateBuilder
    implements
        Builder<WebhookSubscriptionUpdate, WebhookSubscriptionUpdateBuilder> {
  _$WebhookSubscriptionUpdate? _$v;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<String>? _eventTypes;
  ListBuilder<String> get eventTypes =>
      _$this._eventTypes ??= ListBuilder<String>();
  set eventTypes(ListBuilder<String>? eventTypes) =>
      _$this._eventTypes = eventTypes;

  WebhookSubscriptionUpdateStateEnum? _state;
  WebhookSubscriptionUpdateStateEnum? get state => _$this._state;
  set state(WebhookSubscriptionUpdateStateEnum? state) => _$this._state = state;

  bool? _rotateHmacSecret;
  bool? get rotateHmacSecret => _$this._rotateHmacSecret;
  set rotateHmacSecret(bool? rotateHmacSecret) =>
      _$this._rotateHmacSecret = rotateHmacSecret;

  WebhookSubscriptionUpdateBuilder() {
    WebhookSubscriptionUpdate._defaults(this);
  }

  WebhookSubscriptionUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _url = $v.url;
      _eventTypes = $v.eventTypes?.toBuilder();
      _state = $v.state;
      _rotateHmacSecret = $v.rotateHmacSecret;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookSubscriptionUpdate other) {
    _$v = other as _$WebhookSubscriptionUpdate;
  }

  @override
  void update(void Function(WebhookSubscriptionUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookSubscriptionUpdate build() => _build();

  _$WebhookSubscriptionUpdate _build() {
    _$WebhookSubscriptionUpdate _$result;
    try {
      _$result =
          _$v ??
          _$WebhookSubscriptionUpdate._(
            url: url,
            eventTypes: _eventTypes?.build(),
            state: state,
            rotateHmacSecret: rotateHmacSecret,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'eventTypes';
        _eventTypes?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookSubscriptionUpdate',
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
