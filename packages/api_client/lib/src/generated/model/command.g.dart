// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Command extends Command {
  @override
  final String id;
  @override
  final String ownerPrincipalId;
  @override
  final String name;
  @override
  final String description;
  @override
  final TargetKind scopeKind;
  @override
  final String? scopeId;
  @override
  final BuiltList<CommandOption> options;
  @override
  final CommandLevel requiredLevel;
  @override
  final bool control;
  @override
  final int version;

  factory _$Command([void Function(CommandBuilder)? updates]) =>
      (CommandBuilder()..update(updates))._build();

  _$Command._({
    required this.id,
    required this.ownerPrincipalId,
    required this.name,
    required this.description,
    required this.scopeKind,
    this.scopeId,
    required this.options,
    required this.requiredLevel,
    required this.control,
    required this.version,
  }) : super._();
  @override
  Command rebuild(void Function(CommandBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandBuilder toBuilder() => CommandBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Command &&
        id == other.id &&
        ownerPrincipalId == other.ownerPrincipalId &&
        name == other.name &&
        description == other.description &&
        scopeKind == other.scopeKind &&
        scopeId == other.scopeId &&
        options == other.options &&
        requiredLevel == other.requiredLevel &&
        control == other.control &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, ownerPrincipalId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, scopeKind.hashCode);
    _$hash = $jc(_$hash, scopeId.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jc(_$hash, requiredLevel.hashCode);
    _$hash = $jc(_$hash, control.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Command')
          ..add('id', id)
          ..add('ownerPrincipalId', ownerPrincipalId)
          ..add('name', name)
          ..add('description', description)
          ..add('scopeKind', scopeKind)
          ..add('scopeId', scopeId)
          ..add('options', options)
          ..add('requiredLevel', requiredLevel)
          ..add('control', control)
          ..add('version', version))
        .toString();
  }
}

class CommandBuilder implements Builder<Command, CommandBuilder> {
  _$Command? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _ownerPrincipalId;
  String? get ownerPrincipalId => _$this._ownerPrincipalId;
  set ownerPrincipalId(String? ownerPrincipalId) =>
      _$this._ownerPrincipalId = ownerPrincipalId;

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

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  CommandBuilder() {
    Command._defaults(this);
  }

  CommandBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _ownerPrincipalId = $v.ownerPrincipalId;
      _name = $v.name;
      _description = $v.description;
      _scopeKind = $v.scopeKind;
      _scopeId = $v.scopeId;
      _options = $v.options.toBuilder();
      _requiredLevel = $v.requiredLevel;
      _control = $v.control;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Command other) {
    _$v = other as _$Command;
  }

  @override
  void update(void Function(CommandBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Command build() => _build();

  _$Command _build() {
    _$Command _$result;
    try {
      _$result =
          _$v ??
          _$Command._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Command', 'id'),
            ownerPrincipalId: BuiltValueNullFieldError.checkNotNull(
              ownerPrincipalId,
              r'Command',
              'ownerPrincipalId',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'Command',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'Command',
              'description',
            ),
            scopeKind: BuiltValueNullFieldError.checkNotNull(
              scopeKind,
              r'Command',
              'scopeKind',
            ),
            scopeId: scopeId,
            options: options.build(),
            requiredLevel: BuiltValueNullFieldError.checkNotNull(
              requiredLevel,
              r'Command',
              'requiredLevel',
            ),
            control: BuiltValueNullFieldError.checkNotNull(
              control,
              r'Command',
              'control',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Command',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        options.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Command',
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
