// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_requested_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentInterviewRequestedPayload extends AgentInterviewRequestedPayload {
  @override
  final String requestId;
  @override
  final BuiltList<AgentInterviewTurn> turns;
  @override
  final bool wantDraft;
  @override
  final BuiltList<AgentInterviewCatalogueEntry> catalogue;
  @override
  final AgentInterviewLimits limits;
  @override
  final DateTime expiresAt;

  factory _$AgentInterviewRequestedPayload([
    void Function(AgentInterviewRequestedPayloadBuilder)? updates,
  ]) => (AgentInterviewRequestedPayloadBuilder()..update(updates))._build();

  _$AgentInterviewRequestedPayload._({
    required this.requestId,
    required this.turns,
    required this.wantDraft,
    required this.catalogue,
    required this.limits,
    required this.expiresAt,
  }) : super._();
  @override
  AgentInterviewRequestedPayload rebuild(
    void Function(AgentInterviewRequestedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewRequestedPayloadBuilder toBuilder() =>
      AgentInterviewRequestedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewRequestedPayload &&
        requestId == other.requestId &&
        turns == other.turns &&
        wantDraft == other.wantDraft &&
        catalogue == other.catalogue &&
        limits == other.limits &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requestId.hashCode);
    _$hash = $jc(_$hash, turns.hashCode);
    _$hash = $jc(_$hash, wantDraft.hashCode);
    _$hash = $jc(_$hash, catalogue.hashCode);
    _$hash = $jc(_$hash, limits.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewRequestedPayload')
          ..add('requestId', requestId)
          ..add('turns', turns)
          ..add('wantDraft', wantDraft)
          ..add('catalogue', catalogue)
          ..add('limits', limits)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class AgentInterviewRequestedPayloadBuilder
    implements
        Builder<
          AgentInterviewRequestedPayload,
          AgentInterviewRequestedPayloadBuilder
        > {
  _$AgentInterviewRequestedPayload? _$v;

  String? _requestId;
  String? get requestId => _$this._requestId;
  set requestId(String? requestId) => _$this._requestId = requestId;

  ListBuilder<AgentInterviewTurn>? _turns;
  ListBuilder<AgentInterviewTurn> get turns =>
      _$this._turns ??= ListBuilder<AgentInterviewTurn>();
  set turns(ListBuilder<AgentInterviewTurn>? turns) => _$this._turns = turns;

  bool? _wantDraft;
  bool? get wantDraft => _$this._wantDraft;
  set wantDraft(bool? wantDraft) => _$this._wantDraft = wantDraft;

  ListBuilder<AgentInterviewCatalogueEntry>? _catalogue;
  ListBuilder<AgentInterviewCatalogueEntry> get catalogue =>
      _$this._catalogue ??= ListBuilder<AgentInterviewCatalogueEntry>();
  set catalogue(ListBuilder<AgentInterviewCatalogueEntry>? catalogue) =>
      _$this._catalogue = catalogue;

  AgentInterviewLimitsBuilder? _limits;
  AgentInterviewLimitsBuilder get limits =>
      _$this._limits ??= AgentInterviewLimitsBuilder();
  set limits(AgentInterviewLimitsBuilder? limits) => _$this._limits = limits;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  AgentInterviewRequestedPayloadBuilder() {
    AgentInterviewRequestedPayload._defaults(this);
  }

  AgentInterviewRequestedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requestId = $v.requestId;
      _turns = $v.turns.toBuilder();
      _wantDraft = $v.wantDraft;
      _catalogue = $v.catalogue.toBuilder();
      _limits = $v.limits.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewRequestedPayload other) {
    _$v = other as _$AgentInterviewRequestedPayload;
  }

  @override
  void update(void Function(AgentInterviewRequestedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewRequestedPayload build() => _build();

  _$AgentInterviewRequestedPayload _build() {
    _$AgentInterviewRequestedPayload _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewRequestedPayload._(
            requestId: BuiltValueNullFieldError.checkNotNull(
              requestId,
              r'AgentInterviewRequestedPayload',
              'requestId',
            ),
            turns: turns.build(),
            wantDraft: BuiltValueNullFieldError.checkNotNull(
              wantDraft,
              r'AgentInterviewRequestedPayload',
              'wantDraft',
            ),
            catalogue: catalogue.build(),
            limits: limits.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'AgentInterviewRequestedPayload',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'turns';
        turns.build();

        _$failedField = 'catalogue';
        catalogue.build();
        _$failedField = 'limits';
        limits.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewRequestedPayload',
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
