// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_option_value.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandOptionValue extends CommandOptionValue {
  @override
  final String name;
  @override
  final String value;

  factory _$CommandOptionValue([
    void Function(CommandOptionValueBuilder)? updates,
  ]) => (CommandOptionValueBuilder()..update(updates))._build();

  _$CommandOptionValue._({required this.name, required this.value}) : super._();
  @override
  CommandOptionValue rebuild(
    void Function(CommandOptionValueBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CommandOptionValueBuilder toBuilder() =>
      CommandOptionValueBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandOptionValue &&
        name == other.name &&
        value == other.value;
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
    return (newBuiltValueToStringHelper(r'CommandOptionValue')
          ..add('name', name)
          ..add('value', value))
        .toString();
  }
}

class CommandOptionValueBuilder
    implements Builder<CommandOptionValue, CommandOptionValueBuilder> {
  _$CommandOptionValue? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  CommandOptionValueBuilder() {
    CommandOptionValue._defaults(this);
  }

  CommandOptionValueBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandOptionValue other) {
    _$v = other as _$CommandOptionValue;
  }

  @override
  void update(void Function(CommandOptionValueBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandOptionValue build() => _build();

  _$CommandOptionValue _build() {
    final _$result =
        _$v ??
        _$CommandOptionValue._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'CommandOptionValue',
            'name',
          ),
          value: BuiltValueNullFieldError.checkNotNull(
            value,
            r'CommandOptionValue',
            'value',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
