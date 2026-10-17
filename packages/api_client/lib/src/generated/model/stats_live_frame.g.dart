// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_frame.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsLiveFrame extends StatsLiveFrame {
  @override
  final DateTime at;
  @override
  final int intervalMs;
  @override
  final StatsScope scope;
  @override
  final BuiltList<StatsLiveRun> runs;
  @override
  final bool runsTruncated;
  @override
  final BuiltList<StatsLiveAgent> agents;
  @override
  final BuiltList<StatsLiveRate> rates;
  @override
  final BuiltList<StatsLiveSli>? sli;
  @override
  final StatsLiveConnections? connections;

  factory _$StatsLiveFrame([void Function(StatsLiveFrameBuilder)? updates]) =>
      (StatsLiveFrameBuilder()..update(updates))._build();

  _$StatsLiveFrame._({
    required this.at,
    required this.intervalMs,
    required this.scope,
    required this.runs,
    required this.runsTruncated,
    required this.agents,
    required this.rates,
    this.sli,
    this.connections,
  }) : super._();
  @override
  StatsLiveFrame rebuild(void Function(StatsLiveFrameBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveFrameBuilder toBuilder() => StatsLiveFrameBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveFrame &&
        at == other.at &&
        intervalMs == other.intervalMs &&
        scope == other.scope &&
        runs == other.runs &&
        runsTruncated == other.runsTruncated &&
        agents == other.agents &&
        rates == other.rates &&
        sli == other.sli &&
        connections == other.connections;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, at.hashCode);
    _$hash = $jc(_$hash, intervalMs.hashCode);
    _$hash = $jc(_$hash, scope.hashCode);
    _$hash = $jc(_$hash, runs.hashCode);
    _$hash = $jc(_$hash, runsTruncated.hashCode);
    _$hash = $jc(_$hash, agents.hashCode);
    _$hash = $jc(_$hash, rates.hashCode);
    _$hash = $jc(_$hash, sli.hashCode);
    _$hash = $jc(_$hash, connections.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveFrame')
          ..add('at', at)
          ..add('intervalMs', intervalMs)
          ..add('scope', scope)
          ..add('runs', runs)
          ..add('runsTruncated', runsTruncated)
          ..add('agents', agents)
          ..add('rates', rates)
          ..add('sli', sli)
          ..add('connections', connections))
        .toString();
  }
}

class StatsLiveFrameBuilder
    implements Builder<StatsLiveFrame, StatsLiveFrameBuilder> {
  _$StatsLiveFrame? _$v;

  DateTime? _at;
  DateTime? get at => _$this._at;
  set at(DateTime? at) => _$this._at = at;

  int? _intervalMs;
  int? get intervalMs => _$this._intervalMs;
  set intervalMs(int? intervalMs) => _$this._intervalMs = intervalMs;

  StatsScopeBuilder? _scope;
  StatsScopeBuilder get scope => _$this._scope ??= StatsScopeBuilder();
  set scope(StatsScopeBuilder? scope) => _$this._scope = scope;

  ListBuilder<StatsLiveRun>? _runs;
  ListBuilder<StatsLiveRun> get runs =>
      _$this._runs ??= ListBuilder<StatsLiveRun>();
  set runs(ListBuilder<StatsLiveRun>? runs) => _$this._runs = runs;

  bool? _runsTruncated;
  bool? get runsTruncated => _$this._runsTruncated;
  set runsTruncated(bool? runsTruncated) =>
      _$this._runsTruncated = runsTruncated;

  ListBuilder<StatsLiveAgent>? _agents;
  ListBuilder<StatsLiveAgent> get agents =>
      _$this._agents ??= ListBuilder<StatsLiveAgent>();
  set agents(ListBuilder<StatsLiveAgent>? agents) => _$this._agents = agents;

  ListBuilder<StatsLiveRate>? _rates;
  ListBuilder<StatsLiveRate> get rates =>
      _$this._rates ??= ListBuilder<StatsLiveRate>();
  set rates(ListBuilder<StatsLiveRate>? rates) => _$this._rates = rates;

  ListBuilder<StatsLiveSli>? _sli;
  ListBuilder<StatsLiveSli> get sli =>
      _$this._sli ??= ListBuilder<StatsLiveSli>();
  set sli(ListBuilder<StatsLiveSli>? sli) => _$this._sli = sli;

  StatsLiveConnectionsBuilder? _connections;
  StatsLiveConnectionsBuilder get connections =>
      _$this._connections ??= StatsLiveConnectionsBuilder();
  set connections(StatsLiveConnectionsBuilder? connections) =>
      _$this._connections = connections;

  StatsLiveFrameBuilder() {
    StatsLiveFrame._defaults(this);
  }

  StatsLiveFrameBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _at = $v.at;
      _intervalMs = $v.intervalMs;
      _scope = $v.scope.toBuilder();
      _runs = $v.runs.toBuilder();
      _runsTruncated = $v.runsTruncated;
      _agents = $v.agents.toBuilder();
      _rates = $v.rates.toBuilder();
      _sli = $v.sli?.toBuilder();
      _connections = $v.connections?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveFrame other) {
    _$v = other as _$StatsLiveFrame;
  }

  @override
  void update(void Function(StatsLiveFrameBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveFrame build() => _build();

  _$StatsLiveFrame _build() {
    _$StatsLiveFrame _$result;
    try {
      _$result =
          _$v ??
          _$StatsLiveFrame._(
            at: BuiltValueNullFieldError.checkNotNull(
              at,
              r'StatsLiveFrame',
              'at',
            ),
            intervalMs: BuiltValueNullFieldError.checkNotNull(
              intervalMs,
              r'StatsLiveFrame',
              'intervalMs',
            ),
            scope: scope.build(),
            runs: runs.build(),
            runsTruncated: BuiltValueNullFieldError.checkNotNull(
              runsTruncated,
              r'StatsLiveFrame',
              'runsTruncated',
            ),
            agents: agents.build(),
            rates: rates.build(),
            sli: _sli?.build(),
            connections: _connections?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'scope';
        scope.build();
        _$failedField = 'runs';
        runs.build();

        _$failedField = 'agents';
        agents.build();
        _$failedField = 'rates';
        rates.build();
        _$failedField = 'sli';
        _sli?.build();
        _$failedField = 'connections';
        _connections?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsLiveFrame',
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
