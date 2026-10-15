// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'problem_type.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProblemType extends ProblemType {
  @override
  final String code;
  @override
  final int status;
  @override
  final String title;
  @override
  final String description;
  @override
  final String? clientAction;

  factory _$ProblemType([void Function(ProblemTypeBuilder)? updates]) =>
      (ProblemTypeBuilder()..update(updates))._build();

  _$ProblemType._({
    required this.code,
    required this.status,
    required this.title,
    required this.description,
    this.clientAction,
  }) : super._();
  @override
  ProblemType rebuild(void Function(ProblemTypeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProblemTypeBuilder toBuilder() => ProblemTypeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProblemType &&
        code == other.code &&
        status == other.status &&
        title == other.title &&
        description == other.description &&
        clientAction == other.clientAction;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, clientAction.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProblemType')
          ..add('code', code)
          ..add('status', status)
          ..add('title', title)
          ..add('description', description)
          ..add('clientAction', clientAction))
        .toString();
  }
}

class ProblemTypeBuilder implements Builder<ProblemType, ProblemTypeBuilder> {
  _$ProblemType? _$v;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  int? _status;
  int? get status => _$this._status;
  set status(int? status) => _$this._status = status;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _clientAction;
  String? get clientAction => _$this._clientAction;
  set clientAction(String? clientAction) => _$this._clientAction = clientAction;

  ProblemTypeBuilder() {
    ProblemType._defaults(this);
  }

  ProblemTypeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _code = $v.code;
      _status = $v.status;
      _title = $v.title;
      _description = $v.description;
      _clientAction = $v.clientAction;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProblemType other) {
    _$v = other as _$ProblemType;
  }

  @override
  void update(void Function(ProblemTypeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProblemType build() => _build();

  _$ProblemType _build() {
    final _$result =
        _$v ??
        _$ProblemType._(
          code: BuiltValueNullFieldError.checkNotNull(
            code,
            r'ProblemType',
            'code',
          ),
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'ProblemType',
            'status',
          ),
          title: BuiltValueNullFieldError.checkNotNull(
            title,
            r'ProblemType',
            'title',
          ),
          description: BuiltValueNullFieldError.checkNotNull(
            description,
            r'ProblemType',
            'description',
          ),
          clientAction: clientAction,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
