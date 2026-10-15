// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_profile.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostProfileKindEnum _$hostProfileKindEnum_shell =
    const HostProfileKindEnum._('shell');
const HostProfileKindEnum _$hostProfileKindEnum_claude =
    const HostProfileKindEnum._('claude');
const HostProfileKindEnum _$hostProfileKindEnum_command =
    const HostProfileKindEnum._('command');
const HostProfileKindEnum _$hostProfileKindEnum_unknownDefaultOpenApi =
    const HostProfileKindEnum._('unknownDefaultOpenApi');

HostProfileKindEnum _$hostProfileKindEnumValueOf(String name) {
  switch (name) {
    case 'shell':
      return _$hostProfileKindEnum_shell;
    case 'claude':
      return _$hostProfileKindEnum_claude;
    case 'command':
      return _$hostProfileKindEnum_command;
    case 'unknownDefaultOpenApi':
      return _$hostProfileKindEnum_unknownDefaultOpenApi;
    default:
      return _$hostProfileKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostProfileKindEnum> _$hostProfileKindEnumValues =
    BuiltSet<HostProfileKindEnum>(const <HostProfileKindEnum>[
      _$hostProfileKindEnum_shell,
      _$hostProfileKindEnum_claude,
      _$hostProfileKindEnum_command,
      _$hostProfileKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<HostProfileKindEnum> _$hostProfileKindEnumSerializer =
    _$HostProfileKindEnumSerializer();

class _$HostProfileKindEnumSerializer
    implements PrimitiveSerializer<HostProfileKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'shell': 'shell',
    'claude': 'claude',
    'command': 'command',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'shell': 'shell',
    'claude': 'claude',
    'command': 'command',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostProfileKindEnum];
  @override
  final String wireName = 'HostProfileKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostProfileKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostProfileKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostProfileKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostProfile extends HostProfile {
  @override
  final String name;
  @override
  final String displayName;
  @override
  final HostProfileKindEnum kind;

  factory _$HostProfile([void Function(HostProfileBuilder)? updates]) =>
      (HostProfileBuilder()..update(updates))._build();

  _$HostProfile._({
    required this.name,
    required this.displayName,
    required this.kind,
  }) : super._();
  @override
  HostProfile rebuild(void Function(HostProfileBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostProfileBuilder toBuilder() => HostProfileBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostProfile &&
        name == other.name &&
        displayName == other.displayName &&
        kind == other.kind;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostProfile')
          ..add('name', name)
          ..add('displayName', displayName)
          ..add('kind', kind))
        .toString();
  }
}

class HostProfileBuilder implements Builder<HostProfile, HostProfileBuilder> {
  _$HostProfile? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  HostProfileKindEnum? _kind;
  HostProfileKindEnum? get kind => _$this._kind;
  set kind(HostProfileKindEnum? kind) => _$this._kind = kind;

  HostProfileBuilder() {
    HostProfile._defaults(this);
  }

  HostProfileBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _displayName = $v.displayName;
      _kind = $v.kind;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostProfile other) {
    _$v = other as _$HostProfile;
  }

  @override
  void update(void Function(HostProfileBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostProfile build() => _build();

  _$HostProfile _build() {
    final _$result =
        _$v ??
        _$HostProfile._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'HostProfile',
            'name',
          ),
          displayName: BuiltValueNullFieldError.checkNotNull(
            displayName,
            r'HostProfile',
            'displayName',
          ),
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'HostProfile',
            'kind',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
