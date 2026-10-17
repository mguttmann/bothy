// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_agent.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsLiveAgent extends StatsLiveAgent {
  @override
  final String agentId;
  @override
  final AgentRuntimeState presence;
  @override
  final BillingMode billingMode;
  @override
  final BillingMode? reportedBillingMode;
  @override
  final AgentRateLimitState? rateLimit;

  factory _$StatsLiveAgent([void Function(StatsLiveAgentBuilder)? updates]) =>
      (StatsLiveAgentBuilder()..update(updates))._build();

  _$StatsLiveAgent._({
    required this.agentId,
    required this.presence,
    required this.billingMode,
    this.reportedBillingMode,
    this.rateLimit,
  }) : super._();
  @override
  StatsLiveAgent rebuild(void Function(StatsLiveAgentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveAgentBuilder toBuilder() => StatsLiveAgentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveAgent &&
        agentId == other.agentId &&
        presence == other.presence &&
        billingMode == other.billingMode &&
        reportedBillingMode == other.reportedBillingMode &&
        rateLimit == other.rateLimit;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, presence.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, reportedBillingMode.hashCode);
    _$hash = $jc(_$hash, rateLimit.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveAgent')
          ..add('agentId', agentId)
          ..add('presence', presence)
          ..add('billingMode', billingMode)
          ..add('reportedBillingMode', reportedBillingMode)
          ..add('rateLimit', rateLimit))
        .toString();
  }
}

class StatsLiveAgentBuilder
    implements Builder<StatsLiveAgent, StatsLiveAgentBuilder> {
  _$StatsLiveAgent? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  AgentRuntimeState? _presence;
  AgentRuntimeState? get presence => _$this._presence;
  set presence(AgentRuntimeState? presence) => _$this._presence = presence;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  BillingMode? _reportedBillingMode;
  BillingMode? get reportedBillingMode => _$this._reportedBillingMode;
  set reportedBillingMode(BillingMode? reportedBillingMode) =>
      _$this._reportedBillingMode = reportedBillingMode;

  AgentRateLimitStateBuilder? _rateLimit;
  AgentRateLimitStateBuilder get rateLimit =>
      _$this._rateLimit ??= AgentRateLimitStateBuilder();
  set rateLimit(AgentRateLimitStateBuilder? rateLimit) =>
      _$this._rateLimit = rateLimit;

  StatsLiveAgentBuilder() {
    StatsLiveAgent._defaults(this);
  }

  StatsLiveAgentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _presence = $v.presence;
      _billingMode = $v.billingMode;
      _reportedBillingMode = $v.reportedBillingMode;
      _rateLimit = $v.rateLimit?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveAgent other) {
    _$v = other as _$StatsLiveAgent;
  }

  @override
  void update(void Function(StatsLiveAgentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveAgent build() => _build();

  _$StatsLiveAgent _build() {
    _$StatsLiveAgent _$result;
    try {
      _$result =
          _$v ??
          _$StatsLiveAgent._(
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'StatsLiveAgent',
              'agentId',
            ),
            presence: BuiltValueNullFieldError.checkNotNull(
              presence,
              r'StatsLiveAgent',
              'presence',
            ),
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'StatsLiveAgent',
              'billingMode',
            ),
            reportedBillingMode: reportedBillingMode,
            rateLimit: _rateLimit?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'rateLimit';
        _rateLimit?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsLiveAgent',
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
