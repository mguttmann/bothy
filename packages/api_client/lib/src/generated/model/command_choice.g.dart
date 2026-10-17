// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_choice.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandChoice extends CommandChoice {
  @override
  final String name;
  @override
  final String value;

  factory _$CommandChoice([void Function(CommandChoiceBuilder)? updates]) =>
      (CommandChoiceBuilder()..update(updates))._build();

  _$CommandChoice._({required this.name, required this.value}) : super._();
  @override
  CommandChoice rebuild(void Function(CommandChoiceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandChoiceBuilder toBuilder() => CommandChoiceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandChoice && name == other.name && value == other.value;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandChoice')
          ..add('name', name)
          ..add('value', value))
        .toString();
  }
}

class CommandChoiceBuilder
    implements Builder<CommandChoice, CommandChoiceBuilder> {
  _$CommandChoice? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  CommandChoiceBuilder() {
    CommandChoice._defaults(this);
  }

  CommandChoiceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandChoice other) {
    _$v = other as _$CommandChoice;
  }

  @override
  void update(void Function(CommandChoiceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandChoice build() => _build();

  _$CommandChoice _build() {
    final _$result =
        _$v ??
        _$CommandChoice._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'CommandChoice',
            'name',
          ),
          value: BuiltValueNullFieldError.checkNotNull(
            value,
            r'CommandChoice',
            'value',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
