// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'autocomplete_query.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AutocompleteQuery extends AutocompleteQuery {
  @override
  final String? threadId;
  @override
  final String option;
  @override
  final String value;
  @override
  final BuiltList<CommandOptionValue>? options;

  factory _$AutocompleteQuery([
    void Function(AutocompleteQueryBuilder)? updates,
  ]) => (AutocompleteQueryBuilder()..update(updates))._build();

  _$AutocompleteQuery._({
    this.threadId,
    required this.option,
    required this.value,
    this.options,
  }) : super._();
  @override
  AutocompleteQuery rebuild(void Function(AutocompleteQueryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AutocompleteQueryBuilder toBuilder() =>
      AutocompleteQueryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AutocompleteQuery &&
        threadId == other.threadId &&
        option == other.option &&
        value == other.value &&
        options == other.options;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, option.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AutocompleteQuery')
          ..add('threadId', threadId)
          ..add('option', option)
          ..add('value', value)
          ..add('options', options))
        .toString();
  }
}

class AutocompleteQueryBuilder
    implements Builder<AutocompleteQuery, AutocompleteQueryBuilder> {
  _$AutocompleteQuery? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _option;
  String? get option => _$this._option;
  set option(String? option) => _$this._option = option;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  ListBuilder<CommandOptionValue>? _options;
  ListBuilder<CommandOptionValue> get options =>
      _$this._options ??= ListBuilder<CommandOptionValue>();
  set options(ListBuilder<CommandOptionValue>? options) =>
      _$this._options = options;

  AutocompleteQueryBuilder() {
    AutocompleteQuery._defaults(this);
  }

  AutocompleteQueryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _option = $v.option;
      _value = $v.value;
      _options = $v.options?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AutocompleteQuery other) {
    _$v = other as _$AutocompleteQuery;
  }

  @override
  void update(void Function(AutocompleteQueryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AutocompleteQuery build() => _build();

  _$AutocompleteQuery _build() {
    _$AutocompleteQuery _$result;
    try {
      _$result =
          _$v ??
          _$AutocompleteQuery._(
            threadId: threadId,
            option: BuiltValueNullFieldError.checkNotNull(
              option,
              r'AutocompleteQuery',
              'option',
            ),
            value: BuiltValueNullFieldError.checkNotNull(
              value,
              r'AutocompleteQuery',
              'value',
            ),
            options: _options?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        _options?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AutocompleteQuery',
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
