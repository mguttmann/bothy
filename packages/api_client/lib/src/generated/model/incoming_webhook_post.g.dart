// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_post.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const IncomingWebhookPostVersionEnum _$incomingWebhookPostVersionEnum_n4 =
    const IncomingWebhookPostVersionEnum._('n4');
const IncomingWebhookPostVersionEnum
_$incomingWebhookPostVersionEnum_unknownDefaultOpenApi =
    const IncomingWebhookPostVersionEnum._('unknownDefaultOpenApi');

IncomingWebhookPostVersionEnum _$incomingWebhookPostVersionEnumValueOf(
  String name,
) {
  switch (name) {
    case 'n4':
      return _$incomingWebhookPostVersionEnum_n4;
    case 'unknownDefaultOpenApi':
      return _$incomingWebhookPostVersionEnum_unknownDefaultOpenApi;
    default:
      return _$incomingWebhookPostVersionEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<IncomingWebhookPostVersionEnum>
_$incomingWebhookPostVersionEnumValues =
    BuiltSet<IncomingWebhookPostVersionEnum>(
      const <IncomingWebhookPostVersionEnum>[
        _$incomingWebhookPostVersionEnum_n4,
        _$incomingWebhookPostVersionEnum_unknownDefaultOpenApi,
      ],
    );

const IncomingWebhookPostStatusEnum _$incomingWebhookPostStatusEnum_firing =
    const IncomingWebhookPostStatusEnum._('firing');
const IncomingWebhookPostStatusEnum _$incomingWebhookPostStatusEnum_resolved =
    const IncomingWebhookPostStatusEnum._('resolved');
const IncomingWebhookPostStatusEnum
_$incomingWebhookPostStatusEnum_unknownDefaultOpenApi =
    const IncomingWebhookPostStatusEnum._('unknownDefaultOpenApi');

IncomingWebhookPostStatusEnum _$incomingWebhookPostStatusEnumValueOf(
  String name,
) {
  switch (name) {
    case 'firing':
      return _$incomingWebhookPostStatusEnum_firing;
    case 'resolved':
      return _$incomingWebhookPostStatusEnum_resolved;
    case 'unknownDefaultOpenApi':
      return _$incomingWebhookPostStatusEnum_unknownDefaultOpenApi;
    default:
      return _$incomingWebhookPostStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<IncomingWebhookPostStatusEnum>
_$incomingWebhookPostStatusEnumValues = BuiltSet<IncomingWebhookPostStatusEnum>(
  const <IncomingWebhookPostStatusEnum>[
    _$incomingWebhookPostStatusEnum_firing,
    _$incomingWebhookPostStatusEnum_resolved,
    _$incomingWebhookPostStatusEnum_unknownDefaultOpenApi,
  ],
);

Serializer<IncomingWebhookPostVersionEnum>
_$incomingWebhookPostVersionEnumSerializer =
    _$IncomingWebhookPostVersionEnumSerializer();
Serializer<IncomingWebhookPostStatusEnum>
_$incomingWebhookPostStatusEnumSerializer =
    _$IncomingWebhookPostStatusEnumSerializer();

class _$IncomingWebhookPostVersionEnumSerializer
    implements PrimitiveSerializer<IncomingWebhookPostVersionEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'n4': '4',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    '4': 'n4',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[IncomingWebhookPostVersionEnum];
  @override
  final String wireName = 'IncomingWebhookPostVersionEnum';

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookPostVersionEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  IncomingWebhookPostVersionEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => IncomingWebhookPostVersionEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$IncomingWebhookPostStatusEnumSerializer
    implements PrimitiveSerializer<IncomingWebhookPostStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'firing': 'firing',
    'resolved': 'resolved',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'firing': 'firing',
    'resolved': 'resolved',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[IncomingWebhookPostStatusEnum];
  @override
  final String wireName = 'IncomingWebhookPostStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookPostStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  IncomingWebhookPostStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => IncomingWebhookPostStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$IncomingWebhookPost extends IncomingWebhookPost {
  @override
  final String? contentMd;
  @override
  final String? clientMsgId;
  @override
  final String? title;
  @override
  final String? threadId;
  @override
  final IncomingWebhookPostVersionEnum? version;
  @override
  final String? groupKey;
  @override
  final int? truncatedAlerts;
  @override
  final IncomingWebhookPostStatusEnum? status;
  @override
  final String? receiver;
  @override
  final BuiltMap<String, JsonObject?>? groupLabels;
  @override
  final BuiltMap<String, JsonObject?>? commonLabels;
  @override
  final BuiltMap<String, JsonObject?>? commonAnnotations;
  @override
  final String? externalURL;
  @override
  final BuiltList<AlertmanagerAlert>? alerts;

  factory _$IncomingWebhookPost([
    void Function(IncomingWebhookPostBuilder)? updates,
  ]) => (IncomingWebhookPostBuilder()..update(updates))._build();

  _$IncomingWebhookPost._({
    this.contentMd,
    this.clientMsgId,
    this.title,
    this.threadId,
    this.version,
    this.groupKey,
    this.truncatedAlerts,
    this.status,
    this.receiver,
    this.groupLabels,
    this.commonLabels,
    this.commonAnnotations,
    this.externalURL,
    this.alerts,
  }) : super._();
  @override
  IncomingWebhookPost rebuild(
    void Function(IncomingWebhookPostBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookPostBuilder toBuilder() =>
      IncomingWebhookPostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookPost &&
        contentMd == other.contentMd &&
        clientMsgId == other.clientMsgId &&
        title == other.title &&
        threadId == other.threadId &&
        version == other.version &&
        groupKey == other.groupKey &&
        truncatedAlerts == other.truncatedAlerts &&
        status == other.status &&
        receiver == other.receiver &&
        groupLabels == other.groupLabels &&
        commonLabels == other.commonLabels &&
        commonAnnotations == other.commonAnnotations &&
        externalURL == other.externalURL &&
        alerts == other.alerts;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, clientMsgId.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, groupKey.hashCode);
    _$hash = $jc(_$hash, truncatedAlerts.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, receiver.hashCode);
    _$hash = $jc(_$hash, groupLabels.hashCode);
    _$hash = $jc(_$hash, commonLabels.hashCode);
    _$hash = $jc(_$hash, commonAnnotations.hashCode);
    _$hash = $jc(_$hash, externalURL.hashCode);
    _$hash = $jc(_$hash, alerts.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhookPost')
          ..add('contentMd', contentMd)
          ..add('clientMsgId', clientMsgId)
          ..add('title', title)
          ..add('threadId', threadId)
          ..add('version', version)
          ..add('groupKey', groupKey)
          ..add('truncatedAlerts', truncatedAlerts)
          ..add('status', status)
          ..add('receiver', receiver)
          ..add('groupLabels', groupLabels)
          ..add('commonLabels', commonLabels)
          ..add('commonAnnotations', commonAnnotations)
          ..add('externalURL', externalURL)
          ..add('alerts', alerts))
        .toString();
  }
}

class IncomingWebhookPostBuilder
    implements Builder<IncomingWebhookPost, IncomingWebhookPostBuilder> {
  _$IncomingWebhookPost? _$v;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  String? _clientMsgId;
  String? get clientMsgId => _$this._clientMsgId;
  set clientMsgId(String? clientMsgId) => _$this._clientMsgId = clientMsgId;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  IncomingWebhookPostVersionEnum? _version;
  IncomingWebhookPostVersionEnum? get version => _$this._version;
  set version(IncomingWebhookPostVersionEnum? version) =>
      _$this._version = version;

  String? _groupKey;
  String? get groupKey => _$this._groupKey;
  set groupKey(String? groupKey) => _$this._groupKey = groupKey;

  int? _truncatedAlerts;
  int? get truncatedAlerts => _$this._truncatedAlerts;
  set truncatedAlerts(int? truncatedAlerts) =>
      _$this._truncatedAlerts = truncatedAlerts;

  IncomingWebhookPostStatusEnum? _status;
  IncomingWebhookPostStatusEnum? get status => _$this._status;
  set status(IncomingWebhookPostStatusEnum? status) => _$this._status = status;

  String? _receiver;
  String? get receiver => _$this._receiver;
  set receiver(String? receiver) => _$this._receiver = receiver;

  MapBuilder<String, JsonObject?>? _groupLabels;
  MapBuilder<String, JsonObject?> get groupLabels =>
      _$this._groupLabels ??= MapBuilder<String, JsonObject?>();
  set groupLabels(MapBuilder<String, JsonObject?>? groupLabels) =>
      _$this._groupLabels = groupLabels;

  MapBuilder<String, JsonObject?>? _commonLabels;
  MapBuilder<String, JsonObject?> get commonLabels =>
      _$this._commonLabels ??= MapBuilder<String, JsonObject?>();
  set commonLabels(MapBuilder<String, JsonObject?>? commonLabels) =>
      _$this._commonLabels = commonLabels;

  MapBuilder<String, JsonObject?>? _commonAnnotations;
  MapBuilder<String, JsonObject?> get commonAnnotations =>
      _$this._commonAnnotations ??= MapBuilder<String, JsonObject?>();
  set commonAnnotations(MapBuilder<String, JsonObject?>? commonAnnotations) =>
      _$this._commonAnnotations = commonAnnotations;

  String? _externalURL;
  String? get externalURL => _$this._externalURL;
  set externalURL(String? externalURL) => _$this._externalURL = externalURL;

  ListBuilder<AlertmanagerAlert>? _alerts;
  ListBuilder<AlertmanagerAlert> get alerts =>
      _$this._alerts ??= ListBuilder<AlertmanagerAlert>();
  set alerts(ListBuilder<AlertmanagerAlert>? alerts) => _$this._alerts = alerts;

  IncomingWebhookPostBuilder() {
    IncomingWebhookPost._defaults(this);
  }

  IncomingWebhookPostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _contentMd = $v.contentMd;
      _clientMsgId = $v.clientMsgId;
      _title = $v.title;
      _threadId = $v.threadId;
      _version = $v.version;
      _groupKey = $v.groupKey;
      _truncatedAlerts = $v.truncatedAlerts;
      _status = $v.status;
      _receiver = $v.receiver;
      _groupLabels = $v.groupLabels?.toBuilder();
      _commonLabels = $v.commonLabels?.toBuilder();
      _commonAnnotations = $v.commonAnnotations?.toBuilder();
      _externalURL = $v.externalURL;
      _alerts = $v.alerts?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookPost other) {
    _$v = other as _$IncomingWebhookPost;
  }

  @override
  void update(void Function(IncomingWebhookPostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookPost build() => _build();

  _$IncomingWebhookPost _build() {
    _$IncomingWebhookPost _$result;
    try {
      _$result =
          _$v ??
          _$IncomingWebhookPost._(
            contentMd: contentMd,
            clientMsgId: clientMsgId,
            title: title,
            threadId: threadId,
            version: version,
            groupKey: groupKey,
            truncatedAlerts: truncatedAlerts,
            status: status,
            receiver: receiver,
            groupLabels: _groupLabels?.build(),
            commonLabels: _commonLabels?.build(),
            commonAnnotations: _commonAnnotations?.build(),
            externalURL: externalURL,
            alerts: _alerts?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'groupLabels';
        _groupLabels?.build();
        _$failedField = 'commonLabels';
        _commonLabels?.build();
        _$failedField = 'commonAnnotations';
        _commonAnnotations?.build();

        _$failedField = 'alerts';
        _alerts?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IncomingWebhookPost',
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
