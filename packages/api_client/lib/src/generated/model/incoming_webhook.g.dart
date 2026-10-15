// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhook extends IncomingWebhook {
  @override
  final String id;
  @override
  final String appId;
  @override
  final String channelId;
  @override
  final String? threadId;
  @override
  final String name;
  @override
  final DateTime createdAt;
  @override
  final DateTime? lastUsedAt;
  @override
  final DateTime? revokedAt;

  factory _$IncomingWebhook([void Function(IncomingWebhookBuilder)? updates]) =>
      (IncomingWebhookBuilder()..update(updates))._build();

  _$IncomingWebhook._({
    required this.id,
    required this.appId,
    required this.channelId,
    this.threadId,
    required this.name,
    required this.createdAt,
    this.lastUsedAt,
    this.revokedAt,
  }) : super._();
  @override
  IncomingWebhook rebuild(void Function(IncomingWebhookBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookBuilder toBuilder() => IncomingWebhookBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhook &&
        id == other.id &&
        appId == other.appId &&
        channelId == other.channelId &&
        threadId == other.threadId &&
        name == other.name &&
        createdAt == other.createdAt &&
        lastUsedAt == other.lastUsedAt &&
        revokedAt == other.revokedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, appId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, lastUsedAt.hashCode);
    _$hash = $jc(_$hash, revokedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhook')
          ..add('id', id)
          ..add('appId', appId)
          ..add('channelId', channelId)
          ..add('threadId', threadId)
          ..add('name', name)
          ..add('createdAt', createdAt)
          ..add('lastUsedAt', lastUsedAt)
          ..add('revokedAt', revokedAt))
        .toString();
  }
}

class IncomingWebhookBuilder
    implements Builder<IncomingWebhook, IncomingWebhookBuilder> {
  _$IncomingWebhook? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _appId;
  String? get appId => _$this._appId;
  set appId(String? appId) => _$this._appId = appId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _lastUsedAt;
  DateTime? get lastUsedAt => _$this._lastUsedAt;
  set lastUsedAt(DateTime? lastUsedAt) => _$this._lastUsedAt = lastUsedAt;

  DateTime? _revokedAt;
  DateTime? get revokedAt => _$this._revokedAt;
  set revokedAt(DateTime? revokedAt) => _$this._revokedAt = revokedAt;

  IncomingWebhookBuilder() {
    IncomingWebhook._defaults(this);
  }

  IncomingWebhookBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _appId = $v.appId;
      _channelId = $v.channelId;
      _threadId = $v.threadId;
      _name = $v.name;
      _createdAt = $v.createdAt;
      _lastUsedAt = $v.lastUsedAt;
      _revokedAt = $v.revokedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhook other) {
    _$v = other as _$IncomingWebhook;
  }

  @override
  void update(void Function(IncomingWebhookBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhook build() => _build();

  _$IncomingWebhook _build() {
    final _$result =
        _$v ??
        _$IncomingWebhook._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'IncomingWebhook',
            'id',
          ),
          appId: BuiltValueNullFieldError.checkNotNull(
            appId,
            r'IncomingWebhook',
            'appId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'IncomingWebhook',
            'channelId',
          ),
          threadId: threadId,
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'IncomingWebhook',
            'name',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'IncomingWebhook',
            'createdAt',
          ),
          lastUsedAt: lastUsedAt,
          revokedAt: revokedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
