// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_delivery.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookDelivery extends WebhookDelivery {
  @override
  final String id;
  @override
  final String subscriptionId;
  @override
  final String eventId;
  @override
  final int eventSeq;
  @override
  final String? eventType;
  @override
  final int attempt;
  @override
  final WebhookDeliveryStatus status;
  @override
  final int? responseCode;
  @override
  final String? lastError;
  @override
  final DateTime? nextAttemptAt;
  @override
  final DateTime createdAt;
  @override
  final DateTime? deliveredAt;

  factory _$WebhookDelivery([void Function(WebhookDeliveryBuilder)? updates]) =>
      (WebhookDeliveryBuilder()..update(updates))._build();

  _$WebhookDelivery._({
    required this.id,
    required this.subscriptionId,
    required this.eventId,
    required this.eventSeq,
    this.eventType,
    required this.attempt,
    required this.status,
    this.responseCode,
    this.lastError,
    this.nextAttemptAt,
    required this.createdAt,
    this.deliveredAt,
  }) : super._();
  @override
  WebhookDelivery rebuild(void Function(WebhookDeliveryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  WebhookDeliveryBuilder toBuilder() => WebhookDeliveryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookDelivery &&
        id == other.id &&
        subscriptionId == other.subscriptionId &&
        eventId == other.eventId &&
        eventSeq == other.eventSeq &&
        eventType == other.eventType &&
        attempt == other.attempt &&
        status == other.status &&
        responseCode == other.responseCode &&
        lastError == other.lastError &&
        nextAttemptAt == other.nextAttemptAt &&
        createdAt == other.createdAt &&
        deliveredAt == other.deliveredAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, subscriptionId.hashCode);
    _$hash = $jc(_$hash, eventId.hashCode);
    _$hash = $jc(_$hash, eventSeq.hashCode);
    _$hash = $jc(_$hash, eventType.hashCode);
    _$hash = $jc(_$hash, attempt.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, responseCode.hashCode);
    _$hash = $jc(_$hash, lastError.hashCode);
    _$hash = $jc(_$hash, nextAttemptAt.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, deliveredAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebhookDelivery')
          ..add('id', id)
          ..add('subscriptionId', subscriptionId)
          ..add('eventId', eventId)
          ..add('eventSeq', eventSeq)
          ..add('eventType', eventType)
          ..add('attempt', attempt)
          ..add('status', status)
          ..add('responseCode', responseCode)
          ..add('lastError', lastError)
          ..add('nextAttemptAt', nextAttemptAt)
          ..add('createdAt', createdAt)
          ..add('deliveredAt', deliveredAt))
        .toString();
  }
}

class WebhookDeliveryBuilder
    implements Builder<WebhookDelivery, WebhookDeliveryBuilder> {
  _$WebhookDelivery? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _subscriptionId;
  String? get subscriptionId => _$this._subscriptionId;
  set subscriptionId(String? subscriptionId) =>
      _$this._subscriptionId = subscriptionId;

  String? _eventId;
  String? get eventId => _$this._eventId;
  set eventId(String? eventId) => _$this._eventId = eventId;

  int? _eventSeq;
  int? get eventSeq => _$this._eventSeq;
  set eventSeq(int? eventSeq) => _$this._eventSeq = eventSeq;

  String? _eventType;
  String? get eventType => _$this._eventType;
  set eventType(String? eventType) => _$this._eventType = eventType;

  int? _attempt;
  int? get attempt => _$this._attempt;
  set attempt(int? attempt) => _$this._attempt = attempt;

  WebhookDeliveryStatus? _status;
  WebhookDeliveryStatus? get status => _$this._status;
  set status(WebhookDeliveryStatus? status) => _$this._status = status;

  int? _responseCode;
  int? get responseCode => _$this._responseCode;
  set responseCode(int? responseCode) => _$this._responseCode = responseCode;

  String? _lastError;
  String? get lastError => _$this._lastError;
  set lastError(String? lastError) => _$this._lastError = lastError;

  DateTime? _nextAttemptAt;
  DateTime? get nextAttemptAt => _$this._nextAttemptAt;
  set nextAttemptAt(DateTime? nextAttemptAt) =>
      _$this._nextAttemptAt = nextAttemptAt;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _deliveredAt;
  DateTime? get deliveredAt => _$this._deliveredAt;
  set deliveredAt(DateTime? deliveredAt) => _$this._deliveredAt = deliveredAt;

  WebhookDeliveryBuilder() {
    WebhookDelivery._defaults(this);
  }

  WebhookDeliveryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _subscriptionId = $v.subscriptionId;
      _eventId = $v.eventId;
      _eventSeq = $v.eventSeq;
      _eventType = $v.eventType;
      _attempt = $v.attempt;
      _status = $v.status;
      _responseCode = $v.responseCode;
      _lastError = $v.lastError;
      _nextAttemptAt = $v.nextAttemptAt;
      _createdAt = $v.createdAt;
      _deliveredAt = $v.deliveredAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookDelivery other) {
    _$v = other as _$WebhookDelivery;
  }

  @override
  void update(void Function(WebhookDeliveryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookDelivery build() => _build();

  _$WebhookDelivery _build() {
    final _$result =
        _$v ??
        _$WebhookDelivery._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'WebhookDelivery',
            'id',
          ),
          subscriptionId: BuiltValueNullFieldError.checkNotNull(
            subscriptionId,
            r'WebhookDelivery',
            'subscriptionId',
          ),
          eventId: BuiltValueNullFieldError.checkNotNull(
            eventId,
            r'WebhookDelivery',
            'eventId',
          ),
          eventSeq: BuiltValueNullFieldError.checkNotNull(
            eventSeq,
            r'WebhookDelivery',
            'eventSeq',
          ),
          eventType: eventType,
          attempt: BuiltValueNullFieldError.checkNotNull(
            attempt,
            r'WebhookDelivery',
            'attempt',
          ),
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'WebhookDelivery',
            'status',
          ),
          responseCode: responseCode,
          lastError: lastError,
          nextAttemptAt: nextAttemptAt,
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'WebhookDelivery',
            'createdAt',
          ),
          deliveredAt: deliveredAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
