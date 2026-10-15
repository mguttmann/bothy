// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_connections.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsLiveConnections extends StatsLiveConnections {
  @override
  final int user;
  @override
  final int agent;
  @override
  final int app;

  factory _$StatsLiveConnections([
    void Function(StatsLiveConnectionsBuilder)? updates,
  ]) => (StatsLiveConnectionsBuilder()..update(updates))._build();

  _$StatsLiveConnections._({
    required this.user,
    required this.agent,
    required this.app,
  }) : super._();
  @override
  StatsLiveConnections rebuild(
    void Function(StatsLiveConnectionsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatsLiveConnectionsBuilder toBuilder() =>
      StatsLiveConnectionsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveConnections &&
        user == other.user &&
        agent == other.agent &&
        app == other.app;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jc(_$hash, agent.hashCode);
    _$hash = $jc(_$hash, app.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveConnections')
          ..add('user', user)
          ..add('agent', agent)
          ..add('app', app))
        .toString();
  }
}

class StatsLiveConnectionsBuilder
    implements Builder<StatsLiveConnections, StatsLiveConnectionsBuilder> {
  _$StatsLiveConnections? _$v;

  int? _user;
  int? get user => _$this._user;
  set user(int? user) => _$this._user = user;

  int? _agent;
  int? get agent => _$this._agent;
  set agent(int? agent) => _$this._agent = agent;

  int? _app;
  int? get app => _$this._app;
  set app(int? app) => _$this._app = app;

  StatsLiveConnectionsBuilder() {
    StatsLiveConnections._defaults(this);
  }

  StatsLiveConnectionsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user;
      _agent = $v.agent;
      _app = $v.app;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveConnections other) {
    _$v = other as _$StatsLiveConnections;
  }

  @override
  void update(void Function(StatsLiveConnectionsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveConnections build() => _build();

  _$StatsLiveConnections _build() {
    final _$result =
        _$v ??
        _$StatsLiveConnections._(
          user: BuiltValueNullFieldError.checkNotNull(
            user,
            r'StatsLiveConnections',
            'user',
          ),
          agent: BuiltValueNullFieldError.checkNotNull(
            agent,
            r'StatsLiveConnections',
            'agent',
          ),
          app: BuiltValueNullFieldError.checkNotNull(
            app,
            r'StatsLiveConnections',
            'app',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
