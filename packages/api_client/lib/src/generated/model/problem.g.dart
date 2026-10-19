// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'problem.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Problem extends Problem {
  @override
  final String type;
  @override
  final String title;
  @override
  final int status;
  @override
  final String? detail;
  @override
  final String? instance;
  @override
  final String code;
  @override
  final String correlationId;
  @override
  final String? reason;
  @override
  final String? hint;
  @override
  final BuiltList<FieldError>? errors;
  @override
  final BuiltMap<String, JsonObject?>? current;
  @override
  final int? attemptsRemaining;
  @override
  final BuiltList<InvariantViolation>? conflicts;

  factory _$Problem([void Function(ProblemBuilder)? updates]) =>
      (ProblemBuilder()..update(updates))._build();

  _$Problem._({
    required this.type,
    required this.title,
    required this.status,
    this.detail,
    this.instance,
    required this.code,
    required this.correlationId,
    this.reason,
    this.hint,
    this.errors,
    this.current,
    this.attemptsRemaining,
    this.conflicts,
  }) : super._();
  @override
  Problem rebuild(void Function(ProblemBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProblemBuilder toBuilder() => ProblemBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Problem &&
        type == other.type &&
        title == other.title &&
        status == other.status &&
        detail == other.detail &&
        instance == other.instance &&
        code == other.code &&
        correlationId == other.correlationId &&
        reason == other.reason &&
        hint == other.hint &&
        errors == other.errors &&
        current == other.current &&
        attemptsRemaining == other.attemptsRemaining &&
        conflicts == other.conflicts;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, type.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, detail.hashCode);
    _$hash = $jc(_$hash, instance.hashCode);
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, correlationId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, hint.hashCode);
    _$hash = $jc(_$hash, errors.hashCode);
    _$hash = $jc(_$hash, current.hashCode);
    _$hash = $jc(_$hash, attemptsRemaining.hashCode);
    _$hash = $jc(_$hash, conflicts.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Problem')
          ..add('type', type)
          ..add('title', title)
          ..add('status', status)
          ..add('detail', detail)
          ..add('instance', instance)
          ..add('code', code)
          ..add('correlationId', correlationId)
          ..add('reason', reason)
          ..add('hint', hint)
          ..add('errors', errors)
          ..add('current', current)
          ..add('attemptsRemaining', attemptsRemaining)
          ..add('conflicts', conflicts))
        .toString();
  }
}

class ProblemBuilder implements Builder<Problem, ProblemBuilder> {
  _$Problem? _$v;

  String? _type;
  String? get type => _$this._type;
  set type(String? type) => _$this._type = type;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  int? _status;
  int? get status => _$this._status;
  set status(int? status) => _$this._status = status;

  String? _detail;
  String? get detail => _$this._detail;
  set detail(String? detail) => _$this._detail = detail;

  String? _instance;
  String? get instance => _$this._instance;
  set instance(String? instance) => _$this._instance = instance;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  String? _correlationId;
  String? get correlationId => _$this._correlationId;
  set correlationId(String? correlationId) =>
      _$this._correlationId = correlationId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  String? _hint;
  String? get hint => _$this._hint;
  set hint(String? hint) => _$this._hint = hint;

  ListBuilder<FieldError>? _errors;
  ListBuilder<FieldError> get errors =>
      _$this._errors ??= ListBuilder<FieldError>();
  set errors(ListBuilder<FieldError>? errors) => _$this._errors = errors;

  MapBuilder<String, JsonObject?>? _current;
  MapBuilder<String, JsonObject?> get current =>
      _$this._current ??= MapBuilder<String, JsonObject?>();
  set current(MapBuilder<String, JsonObject?>? current) =>
      _$this._current = current;

  int? _attemptsRemaining;
  int? get attemptsRemaining => _$this._attemptsRemaining;
  set attemptsRemaining(int? attemptsRemaining) =>
      _$this._attemptsRemaining = attemptsRemaining;

  ListBuilder<InvariantViolation>? _conflicts;
  ListBuilder<InvariantViolation> get conflicts =>
      _$this._conflicts ??= ListBuilder<InvariantViolation>();
  set conflicts(ListBuilder<InvariantViolation>? conflicts) =>
      _$this._conflicts = conflicts;

  ProblemBuilder() {
    Problem._defaults(this);
  }

  ProblemBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _type = $v.type;
      _title = $v.title;
      _status = $v.status;
      _detail = $v.detail;
      _instance = $v.instance;
      _code = $v.code;
      _correlationId = $v.correlationId;
      _reason = $v.reason;
      _hint = $v.hint;
      _errors = $v.errors?.toBuilder();
      _current = $v.current?.toBuilder();
      _attemptsRemaining = $v.attemptsRemaining;
      _conflicts = $v.conflicts?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Problem other) {
    _$v = other as _$Problem;
  }

  @override
  void update(void Function(ProblemBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Problem build() => _build();

  _$Problem _build() {
    _$Problem _$result;
    try {
      _$result =
          _$v ??
          _$Problem._(
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'Problem',
              'type',
            ),
            title: BuiltValueNullFieldError.checkNotNull(
              title,
              r'Problem',
              'title',
            ),
            status: BuiltValueNullFieldError.checkNotNull(
              status,
              r'Problem',
              'status',
            ),
            detail: detail,
            instance: instance,
            code: BuiltValueNullFieldError.checkNotNull(
              code,
              r'Problem',
              'code',
            ),
            correlationId: BuiltValueNullFieldError.checkNotNull(
              correlationId,
              r'Problem',
              'correlationId',
            ),
            reason: reason,
            hint: hint,
            errors: _errors?.build(),
            current: _current?.build(),
            attemptsRemaining: attemptsRemaining,
            conflicts: _conflicts?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'errors';
        _errors?.build();
        _$failedField = 'current';
        _current?.build();

        _$failedField = 'conflicts';
        _conflicts?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Problem',
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
