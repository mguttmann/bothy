// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'autocomplete_answer.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AutocompleteAnswer extends AutocompleteAnswer {
  @override
  final BuiltList<CommandChoice> choices;

  factory _$AutocompleteAnswer([
    void Function(AutocompleteAnswerBuilder)? updates,
  ]) => (AutocompleteAnswerBuilder()..update(updates))._build();

  _$AutocompleteAnswer._({required this.choices}) : super._();
  @override
  AutocompleteAnswer rebuild(
    void Function(AutocompleteAnswerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AutocompleteAnswerBuilder toBuilder() =>
      AutocompleteAnswerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AutocompleteAnswer && choices == other.choices;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, choices.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AutocompleteAnswer',
    )..add('choices', choices)).toString();
  }
}

class AutocompleteAnswerBuilder
    implements Builder<AutocompleteAnswer, AutocompleteAnswerBuilder> {
  _$AutocompleteAnswer? _$v;

  ListBuilder<CommandChoice>? _choices;
  ListBuilder<CommandChoice> get choices =>
      _$this._choices ??= ListBuilder<CommandChoice>();
  set choices(ListBuilder<CommandChoice>? choices) => _$this._choices = choices;

  AutocompleteAnswerBuilder() {
    AutocompleteAnswer._defaults(this);
  }

  AutocompleteAnswerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _choices = $v.choices.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AutocompleteAnswer other) {
    _$v = other as _$AutocompleteAnswer;
  }

  @override
  void update(void Function(AutocompleteAnswerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AutocompleteAnswer build() => _build();

  _$AutocompleteAnswer _build() {
    _$AutocompleteAnswer _$result;
    try {
      _$result = _$v ?? _$AutocompleteAnswer._(choices: choices.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'choices';
        choices.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AutocompleteAnswer',
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
