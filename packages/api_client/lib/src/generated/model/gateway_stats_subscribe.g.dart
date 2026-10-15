// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_stats_subscribe.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayStatsSubscribe extends GatewayStatsSubscribe {
  @override
  final String? spaceId;

  factory _$GatewayStatsSubscribe([
    void Function(GatewayStatsSubscribeBuilder)? updates,
  ]) => (GatewayStatsSubscribeBuilder()..update(updates))._build();

  _$GatewayStatsSubscribe._({this.spaceId}) : super._();
  @override
  GatewayStatsSubscribe rebuild(
    void Function(GatewayStatsSubscribeBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayStatsSubscribeBuilder toBuilder() =>
      GatewayStatsSubscribeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayStatsSubscribe && spaceId == other.spaceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GatewayStatsSubscribe',
    )..add('spaceId', spaceId)).toString();
  }
}

class GatewayStatsSubscribeBuilder
    implements Builder<GatewayStatsSubscribe, GatewayStatsSubscribeBuilder> {
  _$GatewayStatsSubscribe? _$v;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  GatewayStatsSubscribeBuilder() {
    GatewayStatsSubscribe._defaults(this);
  }

  GatewayStatsSubscribeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _spaceId = $v.spaceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayStatsSubscribe other) {
    _$v = other as _$GatewayStatsSubscribe;
  }

  @override
  void update(void Function(GatewayStatsSubscribeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayStatsSubscribe build() => _build();

  _$GatewayStatsSubscribe _build() {
    final _$result = _$v ?? _$GatewayStatsSubscribe._(spaceId: spaceId);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
