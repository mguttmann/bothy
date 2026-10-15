// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_backend_agent.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsBackendAgent extends StatsBackendAgent {
  @override
  final String agentId;
  @override
  final BillingMode billingMode;
  @override
  final BillingMode? reportedBillingMode;
  @override
  final BuiltList<BillingMode> billingModesSupported;
  @override
  final int switches;
  @override
  final StatsBackendDenials invocationsDenied;
  @override
  final BuiltList<StatsBackendCost>? costs;
  @override
  final AgentRateLimitState? rateLimit;

  factory _$StatsBackendAgent([
    void Function(StatsBackendAgentBuilder)? updates,
  ]) => (StatsBackendAgentBuilder()..update(updates))._build();

  _$StatsBackendAgent._({
    required this.agentId,
    required this.billingMode,
    this.reportedBillingMode,
    required this.billingModesSupported,
    required this.switches,
    required this.invocationsDenied,
    this.costs,
    this.rateLimit,
  }) : super._();
  @override
  StatsBackendAgent rebuild(void Function(StatsBackendAgentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsBackendAgentBuilder toBuilder() =>
      StatsBackendAgentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsBackendAgent &&
        agentId == other.agentId &&
        billingMode == other.billingMode &&
        reportedBillingMode == other.reportedBillingMode &&
        billingModesSupported == other.billingModesSupported &&
        switches == other.switches &&
        invocationsDenied == other.invocationsDenied &&
        costs == other.costs &&
        rateLimit == other.rateLimit;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, reportedBillingMode.hashCode);
    _$hash = $jc(_$hash, billingModesSupported.hashCode);
    _$hash = $jc(_$hash, switches.hashCode);
    _$hash = $jc(_$hash, invocationsDenied.hashCode);
    _$hash = $jc(_$hash, costs.hashCode);
    _$hash = $jc(_$hash, rateLimit.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsBackendAgent')
          ..add('agentId', agentId)
          ..add('billingMode', billingMode)
          ..add('reportedBillingMode', reportedBillingMode)
          ..add('billingModesSupported', billingModesSupported)
          ..add('switches', switches)
          ..add('invocationsDenied', invocationsDenied)
          ..add('costs', costs)
          ..add('rateLimit', rateLimit))
        .toString();
  }
}

class StatsBackendAgentBuilder
    implements Builder<StatsBackendAgent, StatsBackendAgentBuilder> {
  _$StatsBackendAgent? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  BillingMode? _reportedBillingMode;
  BillingMode? get reportedBillingMode => _$this._reportedBillingMode;
  set reportedBillingMode(BillingMode? reportedBillingMode) =>
      _$this._reportedBillingMode = reportedBillingMode;

  ListBuilder<BillingMode>? _billingModesSupported;
  ListBuilder<BillingMode> get billingModesSupported =>
      _$this._billingModesSupported ??= ListBuilder<BillingMode>();
  set billingModesSupported(ListBuilder<BillingMode>? billingModesSupported) =>
      _$this._billingModesSupported = billingModesSupported;

  int? _switches;
  int? get switches => _$this._switches;
  set switches(int? switches) => _$this._switches = switches;

  StatsBackendDenialsBuilder? _invocationsDenied;
  StatsBackendDenialsBuilder get invocationsDenied =>
      _$this._invocationsDenied ??= StatsBackendDenialsBuilder();
  set invocationsDenied(StatsBackendDenialsBuilder? invocationsDenied) =>
      _$this._invocationsDenied = invocationsDenied;

  ListBuilder<StatsBackendCost>? _costs;
  ListBuilder<StatsBackendCost> get costs =>
      _$this._costs ??= ListBuilder<StatsBackendCost>();
  set costs(ListBuilder<StatsBackendCost>? costs) => _$this._costs = costs;

  AgentRateLimitStateBuilder? _rateLimit;
  AgentRateLimitStateBuilder get rateLimit =>
      _$this._rateLimit ??= AgentRateLimitStateBuilder();
  set rateLimit(AgentRateLimitStateBuilder? rateLimit) =>
      _$this._rateLimit = rateLimit;

  StatsBackendAgentBuilder() {
    StatsBackendAgent._defaults(this);
  }

  StatsBackendAgentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _billingMode = $v.billingMode;
      _reportedBillingMode = $v.reportedBillingMode;
      _billingModesSupported = $v.billingModesSupported.toBuilder();
      _switches = $v.switches;
      _invocationsDenied = $v.invocationsDenied.toBuilder();
      _costs = $v.costs?.toBuilder();
      _rateLimit = $v.rateLimit?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsBackendAgent other) {
    _$v = other as _$StatsBackendAgent;
  }

  @override
  void update(void Function(StatsBackendAgentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsBackendAgent build() => _build();

  _$StatsBackendAgent _build() {
    _$StatsBackendAgent _$result;
    try {
      _$result =
          _$v ??
          _$StatsBackendAgent._(
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'StatsBackendAgent',
              'agentId',
            ),
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'StatsBackendAgent',
              'billingMode',
            ),
            reportedBillingMode: reportedBillingMode,
            billingModesSupported: billingModesSupported.build(),
            switches: BuiltValueNullFieldError.checkNotNull(
              switches,
              r'StatsBackendAgent',
              'switches',
            ),
            invocationsDenied: invocationsDenied.build(),
            costs: _costs?.build(),
            rateLimit: _rateLimit?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'billingModesSupported';
        billingModesSupported.build();

        _$failedField = 'invocationsDenied';
        invocationsDenied.build();
        _$failedField = 'costs';
        _costs?.build();
        _$failedField = 'rateLimit';
        _rateLimit?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsBackendAgent',
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
