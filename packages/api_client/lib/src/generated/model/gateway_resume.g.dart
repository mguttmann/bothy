// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_resume.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayResume extends GatewayResume {
  @override
  final String gatewaySessionId;
  @override
  final String? accessToken;
  @override
  final String? apiToken;
  @override
  final String? dpop;
  @override
  final int cursor;
  @override
  final String? syncEpoch;

  factory _$GatewayResume([void Function(GatewayResumeBuilder)? updates]) =>
      (GatewayResumeBuilder()..update(updates))._build();

  _$GatewayResume._({
    required this.gatewaySessionId,
    this.accessToken,
    this.apiToken,
    this.dpop,
    required this.cursor,
    this.syncEpoch,
  }) : super._();
  @override
  GatewayResume rebuild(void Function(GatewayResumeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayResumeBuilder toBuilder() => GatewayResumeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayResume &&
        gatewaySessionId == other.gatewaySessionId &&
        accessToken == other.accessToken &&
        apiToken == other.apiToken &&
        dpop == other.dpop &&
        cursor == other.cursor &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, gatewaySessionId.hashCode);
    _$hash = $jc(_$hash, accessToken.hashCode);
    _$hash = $jc(_$hash, apiToken.hashCode);
    _$hash = $jc(_$hash, dpop.hashCode);
    _$hash = $jc(_$hash, cursor.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayResume')
          ..add('gatewaySessionId', gatewaySessionId)
          ..add('accessToken', accessToken)
          ..add('apiToken', apiToken)
          ..add('dpop', dpop)
          ..add('cursor', cursor)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class GatewayResumeBuilder
    implements Builder<GatewayResume, GatewayResumeBuilder> {
  _$GatewayResume? _$v;

  String? _gatewaySessionId;
  String? get gatewaySessionId => _$this._gatewaySessionId;
  set gatewaySessionId(String? gatewaySessionId) =>
      _$this._gatewaySessionId = gatewaySessionId;

  String? _accessToken;
  String? get accessToken => _$this._accessToken;
  set accessToken(String? accessToken) => _$this._accessToken = accessToken;

  String? _apiToken;
  String? get apiToken => _$this._apiToken;
  set apiToken(String? apiToken) => _$this._apiToken = apiToken;

  String? _dpop;
  String? get dpop => _$this._dpop;
  set dpop(String? dpop) => _$this._dpop = dpop;

  int? _cursor;
  int? get cursor => _$this._cursor;
  set cursor(int? cursor) => _$this._cursor = cursor;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayResumeBuilder() {
    GatewayResume._defaults(this);
  }

  GatewayResumeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _gatewaySessionId = $v.gatewaySessionId;
      _accessToken = $v.accessToken;
      _apiToken = $v.apiToken;
      _dpop = $v.dpop;
      _cursor = $v.cursor;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayResume other) {
    _$v = other as _$GatewayResume;
  }

  @override
  void update(void Function(GatewayResumeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayResume build() => _build();

  _$GatewayResume _build() {
    final _$result =
        _$v ??
        _$GatewayResume._(
          gatewaySessionId: BuiltValueNullFieldError.checkNotNull(
            gatewaySessionId,
            r'GatewayResume',
            'gatewaySessionId',
          ),
          accessToken: accessToken,
          apiToken: apiToken,
          dpop: dpop,
          cursor: BuiltValueNullFieldError.checkNotNull(
            cursor,
            r'GatewayResume',
            'cursor',
          ),
          syncEpoch: syncEpoch,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
