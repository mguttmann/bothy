// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_register.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandRegister extends CommandRegister {
  @override
  final String name;
  @override
  final String description;
  @override
  final TargetKind? scopeKind;
  @override
  final String? scopeId;
  @override
  final BuiltList<CommandOption>? options;
  @override
  final CommandLevel? requiredLevel;
  @override
  final bool? control;

  factory _$CommandRegister([void Function(CommandRegisterBuilder)? updates]) =>
      (CommandRegisterBuilder()..update(updates))._build();

  _$CommandRegister._({
    required this.name,
    required this.description,
    this.scopeKind,
    this.scopeId,
    this.options,
    this.requiredLevel,
    this.control,
  }) : super._();
  @override
  CommandRegister rebuild(void Function(CommandRegisterBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandRegisterBuilder toBuilder() => CommandRegisterBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandRegister &&
        name == other.name &&
        description == other.description &&
        scopeKind == other.scopeKind &&
        scopeId == other.scopeId &&
        options == other.options &&
        requiredLevel == other.requiredLevel &&
        control == other.control;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, scopeKind.hashCode);
    _$hash = $jc(_$hash, scopeId.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jc(_$hash, requiredLevel.hashCode);
    _$hash = $jc(_$hash, control.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandRegister')
          ..add('name', name)
          ..add('description', description)
          ..add('scopeKind', scopeKind)
          ..add('scopeId', scopeId)
          ..add('options', options)
          ..add('requiredLevel', requiredLevel)
          ..add('control', control))
        .toString();
  }
}

class CommandRegisterBuilder
    implements Builder<CommandRegister, CommandRegisterBuilder> {
  _$CommandRegister? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  TargetKind? _scopeKind;
  TargetKind? get scopeKind => _$this._scopeKind;
  set scopeKind(TargetKind? scopeKind) => _$this._scopeKind = scopeKind;

  String? _scopeId;
  String? get scopeId => _$this._scopeId;
  set scopeId(String? scopeId) => _$this._scopeId = scopeId;

  ListBuilder<CommandOption>? _options;
  ListBuilder<CommandOption> get options =>
      _$this._options ??= ListBuilder<CommandOption>();
  set options(ListBuilder<CommandOption>? options) => _$this._options = options;

  CommandLevel? _requiredLevel;
  CommandLevel? get requiredLevel => _$this._requiredLevel;
  set requiredLevel(CommandLevel? requiredLevel) =>
      _$this._requiredLevel = requiredLevel;

  bool? _control;
  bool? get control => _$this._control;
  set control(bool? control) => _$this._control = control;

  CommandRegisterBuilder() {
    CommandRegister._defaults(this);
  }

  CommandRegisterBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _scopeKind = $v.scopeKind;
      _scopeId = $v.scopeId;
      _options = $v.options?.toBuilder();
      _requiredLevel = $v.requiredLevel;
      _control = $v.control;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandRegister other) {
    _$v = other as _$CommandRegister;
  }

  @override
  void update(void Function(CommandRegisterBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandRegister build() => _build();

  _$CommandRegister _build() {
    _$CommandRegister _$result;
    try {
      _$result =
          _$v ??
          _$CommandRegister._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'CommandRegister',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'CommandRegister',
              'description',
            ),
            scopeKind: scopeKind,
            scopeId: scopeId,
            options: _options?.build(),
            requiredLevel: requiredLevel,
            control: control,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        _options?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandRegister',
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
