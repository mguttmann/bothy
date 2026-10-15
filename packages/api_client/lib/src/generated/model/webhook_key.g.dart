// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_key.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const WebhookKeyStateEnum _$webhookKeyStateEnum_next =
    const WebhookKeyStateEnum._('next');
const WebhookKeyStateEnum _$webhookKeyStateEnum_active =
    const WebhookKeyStateEnum._('active');
const WebhookKeyStateEnum _$webhookKeyStateEnum_retiring =
    const WebhookKeyStateEnum._('retiring');
const WebhookKeyStateEnum _$webhookKeyStateEnum_unknownDefaultOpenApi =
    const WebhookKeyStateEnum._('unknownDefaultOpenApi');

WebhookKeyStateEnum _$webhookKeyStateEnumValueOf(String name) {
  switch (name) {
    case 'next':
      return _$webhookKeyStateEnum_next;
    case 'active':
      return _$webhookKeyStateEnum_active;
    case 'retiring':
      return _$webhookKeyStateEnum_retiring;
    case 'unknownDefaultOpenApi':
      return _$webhookKeyStateEnum_unknownDefaultOpenApi;
    default:
      return _$webhookKeyStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<WebhookKeyStateEnum> _$webhookKeyStateEnumValues =
    BuiltSet<WebhookKeyStateEnum>(const <WebhookKeyStateEnum>[
      _$webhookKeyStateEnum_next,
      _$webhookKeyStateEnum_active,
      _$webhookKeyStateEnum_retiring,
      _$webhookKeyStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<WebhookKeyStateEnum> _$webhookKeyStateEnumSerializer =
    _$WebhookKeyStateEnumSerializer();

class _$WebhookKeyStateEnumSerializer
    implements PrimitiveSerializer<WebhookKeyStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'next': 'next',
    'active': 'active',
    'retiring': 'retiring',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'next': 'next',
    'active': 'active',
    'retiring': 'retiring',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[WebhookKeyStateEnum];
  @override
  final String wireName = 'WebhookKeyStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    WebhookKeyStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WebhookKeyStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WebhookKeyStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$WebhookKey extends WebhookKey {
  @override
  final String kid;
  @override
  final String publicKey;
  @override
  final WebhookKeyStateEnum state;
  @override
  final DateTime? notAfter;

  factory _$WebhookKey([void Function(WebhookKeyBuilder)? updates]) =>
      (WebhookKeyBuilder()..update(updates))._build();

  _$WebhookKey._({
    required this.kid,
    required this.publicKey,
    required this.state,
    this.notAfter,
  }) : super._();
  @override
  WebhookKey rebuild(void Function(WebhookKeyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  WebhookKeyBuilder toBuilder() => WebhookKeyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookKey &&
        kid == other.kid &&
        publicKey == other.publicKey &&
        state == other.state &&
        notAfter == other.notAfter;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kid.hashCode);
    _$hash = $jc(_$hash, publicKey.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, notAfter.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookKey')
          ..add('kid', kid)
          ..add('publicKey', publicKey)
          ..add('state', state)
          ..add('notAfter', notAfter))
        .toString();
  }
}

class WebhookKeyBuilder implements Builder<WebhookKey, WebhookKeyBuilder> {
  _$WebhookKey? _$v;

  String? _kid;
  String? get kid => _$this._kid;
  set kid(String? kid) => _$this._kid = kid;

  String? _publicKey;
  String? get publicKey => _$this._publicKey;
  set publicKey(String? publicKey) => _$this._publicKey = publicKey;

  WebhookKeyStateEnum? _state;
  WebhookKeyStateEnum? get state => _$this._state;
  set state(WebhookKeyStateEnum? state) => _$this._state = state;

  DateTime? _notAfter;
  DateTime? get notAfter => _$this._notAfter;
  set notAfter(DateTime? notAfter) => _$this._notAfter = notAfter;

  WebhookKeyBuilder() {
    WebhookKey._defaults(this);
  }

  WebhookKeyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kid = $v.kid;
      _publicKey = $v.publicKey;
      _state = $v.state;
      _notAfter = $v.notAfter;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookKey other) {
    _$v = other as _$WebhookKey;
  }

  @override
  void update(void Function(WebhookKeyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookKey build() => _build();

  _$WebhookKey _build() {
    final _$result =
        _$v ??
        _$WebhookKey._(
          kid: BuiltValueNullFieldError.checkNotNull(kid, r'WebhookKey', 'kid'),
          publicKey: BuiltValueNullFieldError.checkNotNull(
            publicKey,
            r'WebhookKey',
            'publicKey',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'WebhookKey',
            'state',
          ),
          notAfter: notAfter,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
