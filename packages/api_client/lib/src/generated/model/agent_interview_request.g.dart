// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentInterviewRequest extends AgentInterviewRequest {
  @override
  final BuiltList<AgentInterviewTurn> turns;
  @override
  final bool? wantDraft;

  factory _$AgentInterviewRequest([
    void Function(AgentInterviewRequestBuilder)? updates,
  ]) => (AgentInterviewRequestBuilder()..update(updates))._build();

  _$AgentInterviewRequest._({required this.turns, this.wantDraft}) : super._();
  @override
  AgentInterviewRequest rebuild(
    void Function(AgentInterviewRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewRequestBuilder toBuilder() =>
      AgentInterviewRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewRequest &&
        turns == other.turns &&
        wantDraft == other.wantDraft;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, turns.hashCode);
    _$hash = $jc(_$hash, wantDraft.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewRequest')
          ..add('turns', turns)
          ..add('wantDraft', wantDraft))
        .toString();
  }
}

class AgentInterviewRequestBuilder
    implements Builder<AgentInterviewRequest, AgentInterviewRequestBuilder> {
  _$AgentInterviewRequest? _$v;

  ListBuilder<AgentInterviewTurn>? _turns;
  ListBuilder<AgentInterviewTurn> get turns =>
      _$this._turns ??= ListBuilder<AgentInterviewTurn>();
  set turns(ListBuilder<AgentInterviewTurn>? turns) => _$this._turns = turns;

  bool? _wantDraft;
  bool? get wantDraft => _$this._wantDraft;
  set wantDraft(bool? wantDraft) => _$this._wantDraft = wantDraft;

  AgentInterviewRequestBuilder() {
    AgentInterviewRequest._defaults(this);
  }

  AgentInterviewRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _turns = $v.turns.toBuilder();
      _wantDraft = $v.wantDraft;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewRequest other) {
    _$v = other as _$AgentInterviewRequest;
  }

  @override
  void update(void Function(AgentInterviewRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewRequest build() => _build();

  _$AgentInterviewRequest _build() {
    _$AgentInterviewRequest _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewRequest._(turns: turns.build(), wantDraft: wantDraft);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'turns';
        turns.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewRequest',
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
