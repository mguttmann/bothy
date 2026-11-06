// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_answer.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentInterviewAnswerOutcomeEnum
_$agentInterviewAnswerOutcomeEnum_questions =
    const AgentInterviewAnswerOutcomeEnum._('questions');
const AgentInterviewAnswerOutcomeEnum _$agentInterviewAnswerOutcomeEnum_draft =
    const AgentInterviewAnswerOutcomeEnum._('draft');
const AgentInterviewAnswerOutcomeEnum
_$agentInterviewAnswerOutcomeEnum_fallback =
    const AgentInterviewAnswerOutcomeEnum._('fallback');
const AgentInterviewAnswerOutcomeEnum
_$agentInterviewAnswerOutcomeEnum_unknownDefaultOpenApi =
    const AgentInterviewAnswerOutcomeEnum._('unknownDefaultOpenApi');

AgentInterviewAnswerOutcomeEnum _$agentInterviewAnswerOutcomeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'questions':
      return _$agentInterviewAnswerOutcomeEnum_questions;
    case 'draft':
      return _$agentInterviewAnswerOutcomeEnum_draft;
    case 'fallback':
      return _$agentInterviewAnswerOutcomeEnum_fallback;
    case 'unknownDefaultOpenApi':
      return _$agentInterviewAnswerOutcomeEnum_unknownDefaultOpenApi;
    default:
      return _$agentInterviewAnswerOutcomeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInterviewAnswerOutcomeEnum>
_$agentInterviewAnswerOutcomeEnumValues =
    BuiltSet<AgentInterviewAnswerOutcomeEnum>(
      const <AgentInterviewAnswerOutcomeEnum>[
        _$agentInterviewAnswerOutcomeEnum_questions,
        _$agentInterviewAnswerOutcomeEnum_draft,
        _$agentInterviewAnswerOutcomeEnum_fallback,
        _$agentInterviewAnswerOutcomeEnum_unknownDefaultOpenApi,
      ],
    );

const AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnum_builderNotConfigured =
    const AgentInterviewAnswerFallbackReasonEnum._('builderNotConfigured');
const AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnum_builderOffline =
    const AgentInterviewAnswerFallbackReasonEnum._('builderOffline');
const AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnum_builderTimeout =
    const AgentInterviewAnswerFallbackReasonEnum._('builderTimeout');
const AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnum_builderInvalid =
    const AgentInterviewAnswerFallbackReasonEnum._('builderInvalid');
const AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnum_unknownDefaultOpenApi =
    const AgentInterviewAnswerFallbackReasonEnum._('unknownDefaultOpenApi');

AgentInterviewAnswerFallbackReasonEnum
_$agentInterviewAnswerFallbackReasonEnumValueOf(String name) {
  switch (name) {
    case 'builderNotConfigured':
      return _$agentInterviewAnswerFallbackReasonEnum_builderNotConfigured;
    case 'builderOffline':
      return _$agentInterviewAnswerFallbackReasonEnum_builderOffline;
    case 'builderTimeout':
      return _$agentInterviewAnswerFallbackReasonEnum_builderTimeout;
    case 'builderInvalid':
      return _$agentInterviewAnswerFallbackReasonEnum_builderInvalid;
    case 'unknownDefaultOpenApi':
      return _$agentInterviewAnswerFallbackReasonEnum_unknownDefaultOpenApi;
    default:
      return _$agentInterviewAnswerFallbackReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInterviewAnswerFallbackReasonEnum>
_$agentInterviewAnswerFallbackReasonEnumValues =
    BuiltSet<AgentInterviewAnswerFallbackReasonEnum>(
      const <AgentInterviewAnswerFallbackReasonEnum>[
        _$agentInterviewAnswerFallbackReasonEnum_builderNotConfigured,
        _$agentInterviewAnswerFallbackReasonEnum_builderOffline,
        _$agentInterviewAnswerFallbackReasonEnum_builderTimeout,
        _$agentInterviewAnswerFallbackReasonEnum_builderInvalid,
        _$agentInterviewAnswerFallbackReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentInterviewAnswerOutcomeEnum>
_$agentInterviewAnswerOutcomeEnumSerializer =
    _$AgentInterviewAnswerOutcomeEnumSerializer();
Serializer<AgentInterviewAnswerFallbackReasonEnum>
_$agentInterviewAnswerFallbackReasonEnumSerializer =
    _$AgentInterviewAnswerFallbackReasonEnumSerializer();

class _$AgentInterviewAnswerOutcomeEnumSerializer
    implements PrimitiveSerializer<AgentInterviewAnswerOutcomeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'questions': 'questions',
    'draft': 'draft',
    'fallback': 'fallback',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'questions': 'questions',
    'draft': 'draft',
    'fallback': 'fallback',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentInterviewAnswerOutcomeEnum];
  @override
  final String wireName = 'AgentInterviewAnswerOutcomeEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewAnswerOutcomeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInterviewAnswerOutcomeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInterviewAnswerOutcomeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInterviewAnswerFallbackReasonEnumSerializer
    implements PrimitiveSerializer<AgentInterviewAnswerFallbackReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'builderNotConfigured': 'builder_not_configured',
    'builderOffline': 'builder_offline',
    'builderTimeout': 'builder_timeout',
    'builderInvalid': 'builder_invalid',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'builder_not_configured': 'builderNotConfigured',
    'builder_offline': 'builderOffline',
    'builder_timeout': 'builderTimeout',
    'builder_invalid': 'builderInvalid',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentInterviewAnswerFallbackReasonEnum,
  ];
  @override
  final String wireName = 'AgentInterviewAnswerFallbackReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewAnswerFallbackReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInterviewAnswerFallbackReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInterviewAnswerFallbackReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInterviewAnswer extends AgentInterviewAnswer {
  @override
  final AgentInterviewAnswerOutcomeEnum outcome;
  @override
  final BuiltList<String>? questions;
  @override
  final AgentDraft? draft;
  @override
  final BuiltList<FieldError>? errors;
  @override
  final AgentInterviewAnswerFallbackReasonEnum? fallbackReason;
  @override
  final AgentInterviewCost? costEstimate;

  factory _$AgentInterviewAnswer([
    void Function(AgentInterviewAnswerBuilder)? updates,
  ]) => (AgentInterviewAnswerBuilder()..update(updates))._build();

  _$AgentInterviewAnswer._({
    required this.outcome,
    this.questions,
    this.draft,
    this.errors,
    this.fallbackReason,
    this.costEstimate,
  }) : super._();
  @override
  AgentInterviewAnswer rebuild(
    void Function(AgentInterviewAnswerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewAnswerBuilder toBuilder() =>
      AgentInterviewAnswerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewAnswer &&
        outcome == other.outcome &&
        questions == other.questions &&
        draft == other.draft &&
        errors == other.errors &&
        fallbackReason == other.fallbackReason &&
        costEstimate == other.costEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, outcome.hashCode);
    _$hash = $jc(_$hash, questions.hashCode);
    _$hash = $jc(_$hash, draft.hashCode);
    _$hash = $jc(_$hash, errors.hashCode);
    _$hash = $jc(_$hash, fallbackReason.hashCode);
    _$hash = $jc(_$hash, costEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewAnswer')
          ..add('outcome', outcome)
          ..add('questions', questions)
          ..add('draft', draft)
          ..add('errors', errors)
          ..add('fallbackReason', fallbackReason)
          ..add('costEstimate', costEstimate))
        .toString();
  }
}

class AgentInterviewAnswerBuilder
    implements Builder<AgentInterviewAnswer, AgentInterviewAnswerBuilder> {
  _$AgentInterviewAnswer? _$v;

  AgentInterviewAnswerOutcomeEnum? _outcome;
  AgentInterviewAnswerOutcomeEnum? get outcome => _$this._outcome;
  set outcome(AgentInterviewAnswerOutcomeEnum? outcome) =>
      _$this._outcome = outcome;

  ListBuilder<String>? _questions;
  ListBuilder<String> get questions =>
      _$this._questions ??= ListBuilder<String>();
  set questions(ListBuilder<String>? questions) =>
      _$this._questions = questions;

  AgentDraftBuilder? _draft;
  AgentDraftBuilder get draft => _$this._draft ??= AgentDraftBuilder();
  set draft(AgentDraftBuilder? draft) => _$this._draft = draft;

  ListBuilder<FieldError>? _errors;
  ListBuilder<FieldError> get errors =>
      _$this._errors ??= ListBuilder<FieldError>();
  set errors(ListBuilder<FieldError>? errors) => _$this._errors = errors;

  AgentInterviewAnswerFallbackReasonEnum? _fallbackReason;
  AgentInterviewAnswerFallbackReasonEnum? get fallbackReason =>
      _$this._fallbackReason;
  set fallbackReason(AgentInterviewAnswerFallbackReasonEnum? fallbackReason) =>
      _$this._fallbackReason = fallbackReason;

  AgentInterviewCostBuilder? _costEstimate;
  AgentInterviewCostBuilder get costEstimate =>
      _$this._costEstimate ??= AgentInterviewCostBuilder();
  set costEstimate(AgentInterviewCostBuilder? costEstimate) =>
      _$this._costEstimate = costEstimate;

  AgentInterviewAnswerBuilder() {
    AgentInterviewAnswer._defaults(this);
  }

  AgentInterviewAnswerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _outcome = $v.outcome;
      _questions = $v.questions?.toBuilder();
      _draft = $v.draft?.toBuilder();
      _errors = $v.errors?.toBuilder();
      _fallbackReason = $v.fallbackReason;
      _costEstimate = $v.costEstimate?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewAnswer other) {
    _$v = other as _$AgentInterviewAnswer;
  }

  @override
  void update(void Function(AgentInterviewAnswerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewAnswer build() => _build();

  _$AgentInterviewAnswer _build() {
    _$AgentInterviewAnswer _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewAnswer._(
            outcome: BuiltValueNullFieldError.checkNotNull(
              outcome,
              r'AgentInterviewAnswer',
              'outcome',
            ),
            questions: _questions?.build(),
            draft: _draft?.build(),
            errors: _errors?.build(),
            fallbackReason: fallbackReason,
            costEstimate: _costEstimate?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'questions';
        _questions?.build();
        _$failedField = 'draft';
        _draft?.build();
        _$failedField = 'errors';
        _errors?.build();

        _$failedField = 'costEstimate';
        _costEstimate?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewAnswer',
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
