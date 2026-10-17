// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_option.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const CommandOptionTypeEnum _$commandOptionTypeEnum_string =
    const CommandOptionTypeEnum._('string');
const CommandOptionTypeEnum _$commandOptionTypeEnum_integer =
    const CommandOptionTypeEnum._('integer');
const CommandOptionTypeEnum _$commandOptionTypeEnum_boolean =
    const CommandOptionTypeEnum._('boolean');
const CommandOptionTypeEnum _$commandOptionTypeEnum_choice =
    const CommandOptionTypeEnum._('choice');
const CommandOptionTypeEnum _$commandOptionTypeEnum_user =
    const CommandOptionTypeEnum._('user');
const CommandOptionTypeEnum _$commandOptionTypeEnum_file =
    const CommandOptionTypeEnum._('file');
const CommandOptionTypeEnum _$commandOptionTypeEnum_unknownDefaultOpenApi =
    const CommandOptionTypeEnum._('unknownDefaultOpenApi');

CommandOptionTypeEnum _$commandOptionTypeEnumValueOf(String name) {
  switch (name) {
    case 'string':
      return _$commandOptionTypeEnum_string;
    case 'integer':
      return _$commandOptionTypeEnum_integer;
    case 'boolean':
      return _$commandOptionTypeEnum_boolean;
    case 'choice':
      return _$commandOptionTypeEnum_choice;
    case 'user':
      return _$commandOptionTypeEnum_user;
    case 'file':
      return _$commandOptionTypeEnum_file;
    case 'unknownDefaultOpenApi':
      return _$commandOptionTypeEnum_unknownDefaultOpenApi;
    default:
      return _$commandOptionTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<CommandOptionTypeEnum> _$commandOptionTypeEnumValues =
    BuiltSet<CommandOptionTypeEnum>(const <CommandOptionTypeEnum>[
      _$commandOptionTypeEnum_string,
      _$commandOptionTypeEnum_integer,
      _$commandOptionTypeEnum_boolean,
      _$commandOptionTypeEnum_choice,
      _$commandOptionTypeEnum_user,
      _$commandOptionTypeEnum_file,
      _$commandOptionTypeEnum_unknownDefaultOpenApi,
    ]);

Serializer<CommandOptionTypeEnum> _$commandOptionTypeEnumSerializer =
    _$CommandOptionTypeEnumSerializer();

class _$CommandOptionTypeEnumSerializer
    implements PrimitiveSerializer<CommandOptionTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'string': 'string',
    'integer': 'integer',
    'boolean': 'boolean',
    'choice': 'choice',
    'user': 'user',
    'file': 'file',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'string': 'string',
    'integer': 'integer',
    'boolean': 'boolean',
    'choice': 'choice',
    'user': 'user',
    'file': 'file',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[CommandOptionTypeEnum];
  @override
  final String wireName = 'CommandOptionTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    CommandOptionTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  CommandOptionTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => CommandOptionTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$CommandOption extends CommandOption {
  @override
  final String name;
  @override
  final String description;
  @override
  final CommandOptionTypeEnum type;
  @override
  final bool required_;
  @override
  final BuiltList<CommandChoice>? choices;
  @override
  final bool? autocomplete;

  factory _$CommandOption([void Function(CommandOptionBuilder)? updates]) =>
      (CommandOptionBuilder()..update(updates))._build();

  _$CommandOption._({
    required this.name,
    required this.description,
    required this.type,
    required this.required_,
    this.choices,
    this.autocomplete,
  }) : super._();
  @override
  CommandOption rebuild(void Function(CommandOptionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandOptionBuilder toBuilder() => CommandOptionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandOption &&
        name == other.name &&
        description == other.description &&
        type == other.type &&
        required_ == other.required_ &&
        choices == other.choices &&
        autocomplete == other.autocomplete;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, type.hashCode);
    _$hash = $jc(_$hash, required_.hashCode);
    _$hash = $jc(_$hash, choices.hashCode);
    _$hash = $jc(_$hash, autocomplete.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandOption')
          ..add('name', name)
          ..add('description', description)
          ..add('type', type)
          ..add('required_', required_)
          ..add('choices', choices)
          ..add('autocomplete', autocomplete))
        .toString();
  }
}

class CommandOptionBuilder
    implements Builder<CommandOption, CommandOptionBuilder> {
  _$CommandOption? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  CommandOptionTypeEnum? _type;
  CommandOptionTypeEnum? get type => _$this._type;
  set type(CommandOptionTypeEnum? type) => _$this._type = type;

  bool? _required_;
  bool? get required_ => _$this._required_;
  set required_(bool? required_) => _$this._required_ = required_;

  ListBuilder<CommandChoice>? _choices;
  ListBuilder<CommandChoice> get choices =>
      _$this._choices ??= ListBuilder<CommandChoice>();
  set choices(ListBuilder<CommandChoice>? choices) => _$this._choices = choices;

  bool? _autocomplete;
  bool? get autocomplete => _$this._autocomplete;
  set autocomplete(bool? autocomplete) => _$this._autocomplete = autocomplete;

  CommandOptionBuilder() {
    CommandOption._defaults(this);
  }

  CommandOptionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _type = $v.type;
      _required_ = $v.required_;
      _choices = $v.choices?.toBuilder();
      _autocomplete = $v.autocomplete;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandOption other) {
    _$v = other as _$CommandOption;
  }

  @override
  void update(void Function(CommandOptionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandOption build() => _build();

  _$CommandOption _build() {
    _$CommandOption _$result;
    try {
      _$result =
          _$v ??
          _$CommandOption._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'CommandOption',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'CommandOption',
              'description',
            ),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'CommandOption',
              'type',
            ),
            required_: BuiltValueNullFieldError.checkNotNull(
              required_,
              r'CommandOption',
              'required_',
            ),
            choices: _choices?.build(),
            autocomplete: autocomplete,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'choices';
        _choices?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandOption',
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
