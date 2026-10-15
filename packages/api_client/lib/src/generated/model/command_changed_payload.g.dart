// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const CommandChangedPayloadChangeEnum
_$commandChangedPayloadChangeEnum_registered =
    const CommandChangedPayloadChangeEnum._('registered');
const CommandChangedPayloadChangeEnum
_$commandChangedPayloadChangeEnum_updated =
    const CommandChangedPayloadChangeEnum._('updated');
const CommandChangedPayloadChangeEnum
_$commandChangedPayloadChangeEnum_removed =
    const CommandChangedPayloadChangeEnum._('removed');
const CommandChangedPayloadChangeEnum
_$commandChangedPayloadChangeEnum_unknownDefaultOpenApi =
    const CommandChangedPayloadChangeEnum._('unknownDefaultOpenApi');

CommandChangedPayloadChangeEnum _$commandChangedPayloadChangeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'registered':
      return _$commandChangedPayloadChangeEnum_registered;
    case 'updated':
      return _$commandChangedPayloadChangeEnum_updated;
    case 'removed':
      return _$commandChangedPayloadChangeEnum_removed;
    case 'unknownDefaultOpenApi':
      return _$commandChangedPayloadChangeEnum_unknownDefaultOpenApi;
    default:
      return _$commandChangedPayloadChangeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<CommandChangedPayloadChangeEnum>
_$commandChangedPayloadChangeEnumValues =
    BuiltSet<CommandChangedPayloadChangeEnum>(
      const <CommandChangedPayloadChangeEnum>[
        _$commandChangedPayloadChangeEnum_registered,
        _$commandChangedPayloadChangeEnum_updated,
        _$commandChangedPayloadChangeEnum_removed,
        _$commandChangedPayloadChangeEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<CommandChangedPayloadChangeEnum>
_$commandChangedPayloadChangeEnumSerializer =
    _$CommandChangedPayloadChangeEnumSerializer();

class _$CommandChangedPayloadChangeEnumSerializer
    implements PrimitiveSerializer<CommandChangedPayloadChangeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'registered': 'registered',
    'updated': 'updated',
    'removed': 'removed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'registered': 'registered',
    'updated': 'updated',
    'removed': 'removed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[CommandChangedPayloadChangeEnum];
  @override
  final String wireName = 'CommandChangedPayloadChangeEnum';

  @override
  Object serialize(
    Serializers serializers,
    CommandChangedPayloadChangeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  CommandChangedPayloadChangeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => CommandChangedPayloadChangeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$CommandChangedPayload extends CommandChangedPayload {
  @override
  final Command command;
  @override
  final CommandChangedPayloadChangeEnum change;

  factory _$CommandChangedPayload([
    void Function(CommandChangedPayloadBuilder)? updates,
  ]) => (CommandChangedPayloadBuilder()..update(updates))._build();

  _$CommandChangedPayload._({required this.command, required this.change})
    : super._();
  @override
  CommandChangedPayload rebuild(
    void Function(CommandChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CommandChangedPayloadBuilder toBuilder() =>
      CommandChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandChangedPayload &&
        command == other.command &&
        change == other.change;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, command.hashCode);
    _$hash = $jc(_$hash, change.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandChangedPayload')
          ..add('command', command)
          ..add('change', change))
        .toString();
  }
}

class CommandChangedPayloadBuilder
    implements Builder<CommandChangedPayload, CommandChangedPayloadBuilder> {
  _$CommandChangedPayload? _$v;

  CommandBuilder? _command;
  CommandBuilder get command => _$this._command ??= CommandBuilder();
  set command(CommandBuilder? command) => _$this._command = command;

  CommandChangedPayloadChangeEnum? _change;
  CommandChangedPayloadChangeEnum? get change => _$this._change;
  set change(CommandChangedPayloadChangeEnum? change) =>
      _$this._change = change;

  CommandChangedPayloadBuilder() {
    CommandChangedPayload._defaults(this);
  }

  CommandChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _command = $v.command.toBuilder();
      _change = $v.change;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandChangedPayload other) {
    _$v = other as _$CommandChangedPayload;
  }

  @override
  void update(void Function(CommandChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandChangedPayload build() => _build();

  _$CommandChangedPayload _build() {
    _$CommandChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$CommandChangedPayload._(
            command: command.build(),
            change: BuiltValueNullFieldError.checkNotNull(
              change,
              r'CommandChangedPayload',
              'change',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'command';
        command.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandChangedPayload',
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
