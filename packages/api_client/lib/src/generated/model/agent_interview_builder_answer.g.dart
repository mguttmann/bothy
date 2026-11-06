// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_builder_answer.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentInterviewBuilderAnswerKindEnum
_$agentInterviewBuilderAnswerKindEnum_questions =
    const AgentInterviewBuilderAnswerKindEnum._('questions');
const AgentInterviewBuilderAnswerKindEnum
_$agentInterviewBuilderAnswerKindEnum_draft =
    const AgentInterviewBuilderAnswerKindEnum._('draft');
const AgentInterviewBuilderAnswerKindEnum
_$agentInterviewBuilderAnswerKindEnum_unknownDefaultOpenApi =
    const AgentInterviewBuilderAnswerKindEnum._('unknownDefaultOpenApi');

AgentInterviewBuilderAnswerKindEnum
_$agentInterviewBuilderAnswerKindEnumValueOf(String name) {
  switch (name) {
    case 'questions':
      return _$agentInterviewBuilderAnswerKindEnum_questions;
    case 'draft':
      return _$agentInterviewBuilderAnswerKindEnum_draft;
    case 'unknownDefaultOpenApi':
      return _$agentInterviewBuilderAnswerKindEnum_unknownDefaultOpenApi;
    default:
      return _$agentInterviewBuilderAnswerKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInterviewBuilderAnswerKindEnum>
_$agentInterviewBuilderAnswerKindEnumValues =
    BuiltSet<AgentInterviewBuilderAnswerKindEnum>(
      const <AgentInterviewBuilderAnswerKindEnum>[
        _$agentInterviewBuilderAnswerKindEnum_questions,
        _$agentInterviewBuilderAnswerKindEnum_draft,
        _$agentInterviewBuilderAnswerKindEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentInterviewBuilderAnswerKindEnum>
_$agentInterviewBuilderAnswerKindEnumSerializer =
    _$AgentInterviewBuilderAnswerKindEnumSerializer();

class _$AgentInterviewBuilderAnswerKindEnumSerializer
    implements PrimitiveSerializer<AgentInterviewBuilderAnswerKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'questions': 'questions',
    'draft': 'draft',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'questions': 'questions',
    'draft': 'draft',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentInterviewBuilderAnswerKindEnum,
  ];
  @override
  final String wireName = 'AgentInterviewBuilderAnswerKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewBuilderAnswerKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInterviewBuilderAnswerKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInterviewBuilderAnswerKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInterviewBuilderAnswer extends AgentInterviewBuilderAnswer {
  @override
  final AgentInterviewBuilderAnswerKindEnum kind;
  @override
  final BuiltList<String>? questions;
  @override
  final AgentDraft? draft;
  @override
  final BuiltList<CostReport>? costs;

  factory _$AgentInterviewBuilderAnswer([
    void Function(AgentInterviewBuilderAnswerBuilder)? updates,
  ]) => (AgentInterviewBuilderAnswerBuilder()..update(updates))._build();

  _$AgentInterviewBuilderAnswer._({
    required this.kind,
    this.questions,
    this.draft,
    this.costs,
  }) : super._();
  @override
  AgentInterviewBuilderAnswer rebuild(
    void Function(AgentInterviewBuilderAnswerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewBuilderAnswerBuilder toBuilder() =>
      AgentInterviewBuilderAnswerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewBuilderAnswer &&
        kind == other.kind &&
        questions == other.questions &&
        draft == other.draft &&
        costs == other.costs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, questions.hashCode);
    _$hash = $jc(_$hash, draft.hashCode);
    _$hash = $jc(_$hash, costs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewBuilderAnswer')
          ..add('kind', kind)
          ..add('questions', questions)
          ..add('draft', draft)
          ..add('costs', costs))
        .toString();
  }
}

class AgentInterviewBuilderAnswerBuilder
    implements
        Builder<
          AgentInterviewBuilderAnswer,
          AgentInterviewBuilderAnswerBuilder
        > {
  _$AgentInterviewBuilderAnswer? _$v;

  AgentInterviewBuilderAnswerKindEnum? _kind;
  AgentInterviewBuilderAnswerKindEnum? get kind => _$this._kind;
  set kind(AgentInterviewBuilderAnswerKindEnum? kind) => _$this._kind = kind;

  ListBuilder<String>? _questions;
  ListBuilder<String> get questions =>
      _$this._questions ??= ListBuilder<String>();
  set questions(ListBuilder<String>? questions) =>
      _$this._questions = questions;

  AgentDraftBuilder? _draft;
  AgentDraftBuilder get draft => _$this._draft ??= AgentDraftBuilder();
  set draft(AgentDraftBuilder? draft) => _$this._draft = draft;

  ListBuilder<CostReport>? _costs;
  ListBuilder<CostReport> get costs =>
      _$this._costs ??= ListBuilder<CostReport>();
  set costs(ListBuilder<CostReport>? costs) => _$this._costs = costs;

  AgentInterviewBuilderAnswerBuilder() {
    AgentInterviewBuilderAnswer._defaults(this);
  }

  AgentInterviewBuilderAnswerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _questions = $v.questions?.toBuilder();
      _draft = $v.draft?.toBuilder();
      _costs = $v.costs?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewBuilderAnswer other) {
    _$v = other as _$AgentInterviewBuilderAnswer;
  }

  @override
  void update(void Function(AgentInterviewBuilderAnswerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewBuilderAnswer build() => _build();

  _$AgentInterviewBuilderAnswer _build() {
    _$AgentInterviewBuilderAnswer _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewBuilderAnswer._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'AgentInterviewBuilderAnswer',
              'kind',
            ),
            questions: _questions?.build(),
            draft: _draft?.build(),
            costs: _costs?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'questions';
        _questions?.build();
        _$failedField = 'draft';
        _draft?.build();
        _$failedField = 'costs';
        _costs?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewBuilderAnswer',
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
