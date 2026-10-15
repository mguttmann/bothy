// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_subscription.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const WebhookSubscriptionStateEnum _$webhookSubscriptionStateEnum_active =
    const WebhookSubscriptionStateEnum._('active');
const WebhookSubscriptionStateEnum _$webhookSubscriptionStateEnum_paused =
    const WebhookSubscriptionStateEnum._('paused');
const WebhookSubscriptionStateEnum _$webhookSubscriptionStateEnum_disabled =
    const WebhookSubscriptionStateEnum._('disabled');
const WebhookSubscriptionStateEnum
_$webhookSubscriptionStateEnum_unknownDefaultOpenApi =
    const WebhookSubscriptionStateEnum._('unknownDefaultOpenApi');

WebhookSubscriptionStateEnum _$webhookSubscriptionStateEnumValueOf(
  String name,
) {
  switch (name) {
    case 'active':
      return _$webhookSubscriptionStateEnum_active;
    case 'paused':
      return _$webhookSubscriptionStateEnum_paused;
    case 'disabled':
      return _$webhookSubscriptionStateEnum_disabled;
    case 'unknownDefaultOpenApi':
      return _$webhookSubscriptionStateEnum_unknownDefaultOpenApi;
    default:
      return _$webhookSubscriptionStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<WebhookSubscriptionStateEnum>
_$webhookSubscriptionStateEnumValues = BuiltSet<WebhookSubscriptionStateEnum>(
  const <WebhookSubscriptionStateEnum>[
    _$webhookSubscriptionStateEnum_active,
    _$webhookSubscriptionStateEnum_paused,
    _$webhookSubscriptionStateEnum_disabled,
    _$webhookSubscriptionStateEnum_unknownDefaultOpenApi,
  ],
);

const WebhookSubscriptionSignatureSchemesEnum
_$webhookSubscriptionSignatureSchemesEnum_v1a =
    const WebhookSubscriptionSignatureSchemesEnum._('v1a');
const WebhookSubscriptionSignatureSchemesEnum
_$webhookSubscriptionSignatureSchemesEnum_v1 =
    const WebhookSubscriptionSignatureSchemesEnum._('v1');
const WebhookSubscriptionSignatureSchemesEnum
_$webhookSubscriptionSignatureSchemesEnum_unknownDefaultOpenApi =
    const WebhookSubscriptionSignatureSchemesEnum._('unknownDefaultOpenApi');

WebhookSubscriptionSignatureSchemesEnum
_$webhookSubscriptionSignatureSchemesEnumValueOf(String name) {
  switch (name) {
    case 'v1a':
      return _$webhookSubscriptionSignatureSchemesEnum_v1a;
    case 'v1':
      return _$webhookSubscriptionSignatureSchemesEnum_v1;
    case 'unknownDefaultOpenApi':
      return _$webhookSubscriptionSignatureSchemesEnum_unknownDefaultOpenApi;
    default:
      return _$webhookSubscriptionSignatureSchemesEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<WebhookSubscriptionSignatureSchemesEnum>
_$webhookSubscriptionSignatureSchemesEnumValues =
    BuiltSet<WebhookSubscriptionSignatureSchemesEnum>(
      const <WebhookSubscriptionSignatureSchemesEnum>[
        _$webhookSubscriptionSignatureSchemesEnum_v1a,
        _$webhookSubscriptionSignatureSchemesEnum_v1,
        _$webhookSubscriptionSignatureSchemesEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<WebhookSubscriptionStateEnum>
_$webhookSubscriptionStateEnumSerializer =
    _$WebhookSubscriptionStateEnumSerializer();
Serializer<WebhookSubscriptionSignatureSchemesEnum>
_$webhookSubscriptionSignatureSchemesEnumSerializer =
    _$WebhookSubscriptionSignatureSchemesEnumSerializer();

class _$WebhookSubscriptionStateEnumSerializer
    implements PrimitiveSerializer<WebhookSubscriptionStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'paused': 'paused',
    'disabled': 'disabled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'paused': 'paused',
    'disabled': 'disabled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[WebhookSubscriptionStateEnum];
  @override
  final String wireName = 'WebhookSubscriptionStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WebhookSubscriptionStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WebhookSubscriptionStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$WebhookSubscriptionSignatureSchemesEnumSerializer
    implements PrimitiveSerializer<WebhookSubscriptionSignatureSchemesEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'v1a': 'v1a',
    'v1': 'v1',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'v1a': 'v1a',
    'v1': 'v1',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    WebhookSubscriptionSignatureSchemesEnum,
  ];
  @override
  final String wireName = 'WebhookSubscriptionSignatureSchemesEnum';

  @override
  Object serialize(
    Serializers serializers,
    WebhookSubscriptionSignatureSchemesEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WebhookSubscriptionSignatureSchemesEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WebhookSubscriptionSignatureSchemesEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$WebhookSubscription extends WebhookSubscription {
  @override
  final String id;
  @override
  final String? appId;
  @override
  final String? agentId;
  @override
  final String url;
  @override
  final BuiltList<String> eventTypes;
  @override
  final WebhookSubscriptionStateEnum state;
  @override
  final int consecutiveFailures;
  @override
  final BuiltList<WebhookSubscriptionSignatureSchemesEnum> signatureSchemes;
  @override
  final String signingKeyKid;
  @override
  final DateTime createdAt;
  @override
  final DateTime? disabledAt;

  factory _$WebhookSubscription([
    void Function(WebhookSubscriptionBuilder)? updates,
  ]) => (WebhookSubscriptionBuilder()..update(updates))._build();

  _$WebhookSubscription._({
    required this.id,
    this.appId,
    this.agentId,
    required this.url,
    required this.eventTypes,
    required this.state,
    required this.consecutiveFailures,
    required this.signatureSchemes,
    required this.signingKeyKid,
    required this.createdAt,
    this.disabledAt,
  }) : super._();
  @override
  WebhookSubscription rebuild(
    void Function(WebhookSubscriptionBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  WebhookSubscriptionBuilder toBuilder() =>
      WebhookSubscriptionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookSubscription &&
        id == other.id &&
        appId == other.appId &&
        agentId == other.agentId &&
        url == other.url &&
        eventTypes == other.eventTypes &&
        state == other.state &&
        consecutiveFailures == other.consecutiveFailures &&
        signatureSchemes == other.signatureSchemes &&
        signingKeyKid == other.signingKeyKid &&
        createdAt == other.createdAt &&
        disabledAt == other.disabledAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, appId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, eventTypes.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, consecutiveFailures.hashCode);
    _$hash = $jc(_$hash, signatureSchemes.hashCode);
    _$hash = $jc(_$hash, signingKeyKid.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, disabledAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookSubscription')
          ..add('id', id)
          ..add('appId', appId)
          ..add('agentId', agentId)
          ..add('url', url)
          ..add('eventTypes', eventTypes)
          ..add('state', state)
          ..add('consecutiveFailures', consecutiveFailures)
          ..add('signatureSchemes', signatureSchemes)
          ..add('signingKeyKid', signingKeyKid)
          ..add('createdAt', createdAt)
          ..add('disabledAt', disabledAt))
        .toString();
  }
}

class WebhookSubscriptionBuilder
    implements Builder<WebhookSubscription, WebhookSubscriptionBuilder> {
  _$WebhookSubscription? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _appId;
  String? get appId => _$this._appId;
  set appId(String? appId) => _$this._appId = appId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<String>? _eventTypes;
  ListBuilder<String> get eventTypes =>
      _$this._eventTypes ??= ListBuilder<String>();
  set eventTypes(ListBuilder<String>? eventTypes) =>
      _$this._eventTypes = eventTypes;

  WebhookSubscriptionStateEnum? _state;
  WebhookSubscriptionStateEnum? get state => _$this._state;
  set state(WebhookSubscriptionStateEnum? state) => _$this._state = state;

  int? _consecutiveFailures;
  int? get consecutiveFailures => _$this._consecutiveFailures;
  set consecutiveFailures(int? consecutiveFailures) =>
      _$this._consecutiveFailures = consecutiveFailures;

  ListBuilder<WebhookSubscriptionSignatureSchemesEnum>? _signatureSchemes;
  ListBuilder<WebhookSubscriptionSignatureSchemesEnum> get signatureSchemes =>
      _$this._signatureSchemes ??=
          ListBuilder<WebhookSubscriptionSignatureSchemesEnum>();
  set signatureSchemes(
    ListBuilder<WebhookSubscriptionSignatureSchemesEnum>? signatureSchemes,
  ) => _$this._signatureSchemes = signatureSchemes;

  String? _signingKeyKid;
  String? get signingKeyKid => _$this._signingKeyKid;
  set signingKeyKid(String? signingKeyKid) =>
      _$this._signingKeyKid = signingKeyKid;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _disabledAt;
  DateTime? get disabledAt => _$this._disabledAt;
  set disabledAt(DateTime? disabledAt) => _$this._disabledAt = disabledAt;

  WebhookSubscriptionBuilder() {
    WebhookSubscription._defaults(this);
  }

  WebhookSubscriptionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _appId = $v.appId;
      _agentId = $v.agentId;
      _url = $v.url;
      _eventTypes = $v.eventTypes.toBuilder();
      _state = $v.state;
      _consecutiveFailures = $v.consecutiveFailures;
      _signatureSchemes = $v.signatureSchemes.toBuilder();
      _signingKeyKid = $v.signingKeyKid;
      _createdAt = $v.createdAt;
      _disabledAt = $v.disabledAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookSubscription other) {
    _$v = other as _$WebhookSubscription;
  }

  @override
  void update(void Function(WebhookSubscriptionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookSubscription build() => _build();

  _$WebhookSubscription _build() {
    _$WebhookSubscription _$result;
    try {
      _$result =
          _$v ??
          _$WebhookSubscription._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'WebhookSubscription',
              'id',
            ),
            appId: appId,
            agentId: agentId,
            url: BuiltValueNullFieldError.checkNotNull(
              url,
              r'WebhookSubscription',
              'url',
            ),
            eventTypes: eventTypes.build(),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'WebhookSubscription',
              'state',
            ),
            consecutiveFailures: BuiltValueNullFieldError.checkNotNull(
              consecutiveFailures,
              r'WebhookSubscription',
              'consecutiveFailures',
            ),
            signatureSchemes: signatureSchemes.build(),
            signingKeyKid: BuiltValueNullFieldError.checkNotNull(
              signingKeyKid,
              r'WebhookSubscription',
              'signingKeyKid',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'WebhookSubscription',
              'createdAt',
            ),
            disabledAt: disabledAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'eventTypes';
        eventTypes.build();

        _$failedField = 'signatureSchemes';
        signatureSchemes.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookSubscription',
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
