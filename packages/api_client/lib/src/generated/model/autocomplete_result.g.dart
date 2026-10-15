// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'autocomplete_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AutocompleteResult extends AutocompleteResult {
  @override
  final BuiltList<CommandChoice> choices;

  factory _$AutocompleteResult([
    void Function(AutocompleteResultBuilder)? updates,
  ]) => (AutocompleteResultBuilder()..update(updates))._build();

  _$AutocompleteResult._({required this.choices}) : super._();
  @override
  AutocompleteResult rebuild(
    void Function(AutocompleteResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AutocompleteResultBuilder toBuilder() =>
      AutocompleteResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AutocompleteResult && choices == other.choices;
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
      r'AutocompleteResult',
    )..add('choices', choices)).toString();
  }
}

class AutocompleteResultBuilder
    implements Builder<AutocompleteResult, AutocompleteResultBuilder> {
  _$AutocompleteResult? _$v;

  ListBuilder<CommandChoice>? _choices;
  ListBuilder<CommandChoice> get choices =>
      _$this._choices ??= ListBuilder<CommandChoice>();
  set choices(ListBuilder<CommandChoice>? choices) => _$this._choices = choices;

  AutocompleteResultBuilder() {
    AutocompleteResult._defaults(this);
  }

  AutocompleteResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _choices = $v.choices.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AutocompleteResult other) {
    _$v = other as _$AutocompleteResult;
  }

  @override
  void update(void Function(AutocompleteResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AutocompleteResult build() => _build();

  _$AutocompleteResult _build() {
    _$AutocompleteResult _$result;
    try {
      _$result = _$v ?? _$AutocompleteResult._(choices: choices.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'choices';
        choices.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AutocompleteResult',
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
