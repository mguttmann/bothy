// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_identify.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayIdentify extends GatewayIdentify {
  @override
  final int protocolVersion;
  @override
  final String? accessToken;
  @override
  final String? apiToken;
  @override
  final String? dpop;
  @override
  final String? deviceId;
  @override
  final String? consumer;
  @override
  final int? cursor;
  @override
  final String? syncEpoch;
  @override
  final GatewayClientInfo? client;

  factory _$GatewayIdentify([void Function(GatewayIdentifyBuilder)? updates]) =>
      (GatewayIdentifyBuilder()..update(updates))._build();

  _$GatewayIdentify._({
    required this.protocolVersion,
    this.accessToken,
    this.apiToken,
    this.dpop,
    this.deviceId,
    this.consumer,
    this.cursor,
    this.syncEpoch,
    this.client,
  }) : super._();
  @override
  GatewayIdentify rebuild(void Function(GatewayIdentifyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayIdentifyBuilder toBuilder() => GatewayIdentifyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayIdentify &&
        protocolVersion == other.protocolVersion &&
        accessToken == other.accessToken &&
        apiToken == other.apiToken &&
        dpop == other.dpop &&
        deviceId == other.deviceId &&
        consumer == other.consumer &&
        cursor == other.cursor &&
        syncEpoch == other.syncEpoch &&
        client == other.client;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersion.hashCode);
    _$hash = $jc(_$hash, accessToken.hashCode);
    _$hash = $jc(_$hash, apiToken.hashCode);
    _$hash = $jc(_$hash, dpop.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, consumer.hashCode);
    _$hash = $jc(_$hash, cursor.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jc(_$hash, client.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayIdentify')
          ..add('protocolVersion', protocolVersion)
          ..add('accessToken', accessToken)
          ..add('apiToken', apiToken)
          ..add('dpop', dpop)
          ..add('deviceId', deviceId)
          ..add('consumer', consumer)
          ..add('cursor', cursor)
          ..add('syncEpoch', syncEpoch)
          ..add('client', client))
        .toString();
  }
}

class GatewayIdentifyBuilder
    implements Builder<GatewayIdentify, GatewayIdentifyBuilder> {
  _$GatewayIdentify? _$v;

  int? _protocolVersion;
  int? get protocolVersion => _$this._protocolVersion;
  set protocolVersion(int? protocolVersion) =>
      _$this._protocolVersion = protocolVersion;

  String? _accessToken;
  String? get accessToken => _$this._accessToken;
  set accessToken(String? accessToken) => _$this._accessToken = accessToken;

  String? _apiToken;
  String? get apiToken => _$this._apiToken;
  set apiToken(String? apiToken) => _$this._apiToken = apiToken;

  String? _dpop;
  String? get dpop => _$this._dpop;
  set dpop(String? dpop) => _$this._dpop = dpop;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  String? _consumer;
  String? get consumer => _$this._consumer;
  set consumer(String? consumer) => _$this._consumer = consumer;

  int? _cursor;
  int? get cursor => _$this._cursor;
  set cursor(int? cursor) => _$this._cursor = cursor;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayClientInfoBuilder? _client;
  GatewayClientInfoBuilder get client =>
      _$this._client ??= GatewayClientInfoBuilder();
  set client(GatewayClientInfoBuilder? client) => _$this._client = client;

  GatewayIdentifyBuilder() {
    GatewayIdentify._defaults(this);
  }

  GatewayIdentifyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersion = $v.protocolVersion;
      _accessToken = $v.accessToken;
      _apiToken = $v.apiToken;
      _dpop = $v.dpop;
      _deviceId = $v.deviceId;
      _consumer = $v.consumer;
      _cursor = $v.cursor;
      _syncEpoch = $v.syncEpoch;
      _client = $v.client?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayIdentify other) {
    _$v = other as _$GatewayIdentify;
  }

  @override
  void update(void Function(GatewayIdentifyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayIdentify build() => _build();

  _$GatewayIdentify _build() {
    _$GatewayIdentify _$result;
    try {
      _$result =
          _$v ??
          _$GatewayIdentify._(
            protocolVersion: BuiltValueNullFieldError.checkNotNull(
              protocolVersion,
              r'GatewayIdentify',
              'protocolVersion',
            ),
            accessToken: accessToken,
            apiToken: apiToken,
            dpop: dpop,
            deviceId: deviceId,
            consumer: consumer,
            cursor: cursor,
            syncEpoch: syncEpoch,
            client: _client?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'client';
        _client?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayIdentify',
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
