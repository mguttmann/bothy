// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_runtime_status.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentRuntimeStatus extends AgentRuntimeStatus {
  @override
  final AgentRuntimeState state;
  @override
  final BillingMode? reportedBillingMode;
  @override
  final String? runtimeVersion;
  @override
  final DateTime? lastHeartbeatAt;
  @override
  final BuiltList<BillingMode>? billingModesSupported;
  @override
  final ToolClass? reportedToolClass;
  @override
  final bool? billingModeMatches;
  @override
  final AgentRateLimitState? rateLimit;

  factory _$AgentRuntimeStatus([
    void Function(AgentRuntimeStatusBuilder)? updates,
  ]) => (AgentRuntimeStatusBuilder()..update(updates))._build();

  _$AgentRuntimeStatus._({
    required this.state,
    this.reportedBillingMode,
    this.runtimeVersion,
    this.lastHeartbeatAt,
    this.billingModesSupported,
    this.reportedToolClass,
    this.billingModeMatches,
    this.rateLimit,
  }) : super._();
  @override
  AgentRuntimeStatus rebuild(
    void Function(AgentRuntimeStatusBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentRuntimeStatusBuilder toBuilder() =>
      AgentRuntimeStatusBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentRuntimeStatus &&
        state == other.state &&
        reportedBillingMode == other.reportedBillingMode &&
        runtimeVersion == other.runtimeVersion &&
        lastHeartbeatAt == other.lastHeartbeatAt &&
        billingModesSupported == other.billingModesSupported &&
        reportedToolClass == other.reportedToolClass &&
        billingModeMatches == other.billingModeMatches &&
        rateLimit == other.rateLimit;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, reportedBillingMode.hashCode);
    _$hash = $jc(_$hash, runtimeVersion.hashCode);
    _$hash = $jc(_$hash, lastHeartbeatAt.hashCode);
    _$hash = $jc(_$hash, billingModesSupported.hashCode);
    _$hash = $jc(_$hash, reportedToolClass.hashCode);
    _$hash = $jc(_$hash, billingModeMatches.hashCode);
    _$hash = $jc(_$hash, rateLimit.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentRuntimeStatus')
          ..add('state', state)
          ..add('reportedBillingMode', reportedBillingMode)
          ..add('runtimeVersion', runtimeVersion)
          ..add('lastHeartbeatAt', lastHeartbeatAt)
          ..add('billingModesSupported', billingModesSupported)
          ..add('reportedToolClass', reportedToolClass)
          ..add('billingModeMatches', billingModeMatches)
          ..add('rateLimit', rateLimit))
        .toString();
  }
}

class AgentRuntimeStatusBuilder
    implements Builder<AgentRuntimeStatus, AgentRuntimeStatusBuilder> {
  _$AgentRuntimeStatus? _$v;

  AgentRuntimeState? _state;
  AgentRuntimeState? get state => _$this._state;
  set state(AgentRuntimeState? state) => _$this._state = state;

  BillingMode? _reportedBillingMode;
  BillingMode? get reportedBillingMode => _$this._reportedBillingMode;
  set reportedBillingMode(BillingMode? reportedBillingMode) =>
      _$this._reportedBillingMode = reportedBillingMode;

  String? _runtimeVersion;
  String? get runtimeVersion => _$this._runtimeVersion;
  set runtimeVersion(String? runtimeVersion) =>
      _$this._runtimeVersion = runtimeVersion;

  DateTime? _lastHeartbeatAt;
  DateTime? get lastHeartbeatAt => _$this._lastHeartbeatAt;
  set lastHeartbeatAt(DateTime? lastHeartbeatAt) =>
      _$this._lastHeartbeatAt = lastHeartbeatAt;

  ListBuilder<BillingMode>? _billingModesSupported;
  ListBuilder<BillingMode> get billingModesSupported =>
      _$this._billingModesSupported ??= ListBuilder<BillingMode>();
  set billingModesSupported(ListBuilder<BillingMode>? billingModesSupported) =>
      _$this._billingModesSupported = billingModesSupported;

  ToolClass? _reportedToolClass;
  ToolClass? get reportedToolClass => _$this._reportedToolClass;
  set reportedToolClass(ToolClass? reportedToolClass) =>
      _$this._reportedToolClass = reportedToolClass;

  bool? _billingModeMatches;
  bool? get billingModeMatches => _$this._billingModeMatches;
  set billingModeMatches(bool? billingModeMatches) =>
      _$this._billingModeMatches = billingModeMatches;

  AgentRateLimitStateBuilder? _rateLimit;
  AgentRateLimitStateBuilder get rateLimit =>
      _$this._rateLimit ??= AgentRateLimitStateBuilder();
  set rateLimit(AgentRateLimitStateBuilder? rateLimit) =>
      _$this._rateLimit = rateLimit;

  AgentRuntimeStatusBuilder() {
    AgentRuntimeStatus._defaults(this);
  }

  AgentRuntimeStatusBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _reportedBillingMode = $v.reportedBillingMode;
      _runtimeVersion = $v.runtimeVersion;
      _lastHeartbeatAt = $v.lastHeartbeatAt;
      _billingModesSupported = $v.billingModesSupported?.toBuilder();
      _reportedToolClass = $v.reportedToolClass;
      _billingModeMatches = $v.billingModeMatches;
      _rateLimit = $v.rateLimit?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentRuntimeStatus other) {
    _$v = other as _$AgentRuntimeStatus;
  }

  @override
  void update(void Function(AgentRuntimeStatusBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentRuntimeStatus build() => _build();

  _$AgentRuntimeStatus _build() {
    _$AgentRuntimeStatus _$result;
    try {
      _$result =
          _$v ??
          _$AgentRuntimeStatus._(
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'AgentRuntimeStatus',
              'state',
            ),
            reportedBillingMode: reportedBillingMode,
            runtimeVersion: runtimeVersion,
            lastHeartbeatAt: lastHeartbeatAt,
            billingModesSupported: _billingModesSupported?.build(),
            reportedToolClass: reportedToolClass,
            billingModeMatches: billingModeMatches,
            rateLimit: _rateLimit?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'billingModesSupported';
        _billingModesSupported?.build();

        _$failedField = 'rateLimit';
        _rateLimit?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentRuntimeStatus',
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
