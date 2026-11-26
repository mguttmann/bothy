// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostComputerHelperEnum _$hostComputerHelperEnum_connected =
    const HostComputerHelperEnum._('connected');
const HostComputerHelperEnum _$hostComputerHelperEnum_absent =
    const HostComputerHelperEnum._('absent');
const HostComputerHelperEnum _$hostComputerHelperEnum_refused =
    const HostComputerHelperEnum._('refused');
const HostComputerHelperEnum _$hostComputerHelperEnum_unknownDefaultOpenApi =
    const HostComputerHelperEnum._('unknownDefaultOpenApi');

HostComputerHelperEnum _$hostComputerHelperEnumValueOf(String name) {
  switch (name) {
    case 'connected':
      return _$hostComputerHelperEnum_connected;
    case 'absent':
      return _$hostComputerHelperEnum_absent;
    case 'refused':
      return _$hostComputerHelperEnum_refused;
    case 'unknownDefaultOpenApi':
      return _$hostComputerHelperEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerHelperEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerHelperEnum> _$hostComputerHelperEnumValues =
    BuiltSet<HostComputerHelperEnum>(const <HostComputerHelperEnum>[
      _$hostComputerHelperEnum_connected,
      _$hostComputerHelperEnum_absent,
      _$hostComputerHelperEnum_refused,
      _$hostComputerHelperEnum_unknownDefaultOpenApi,
    ]);

const HostComputerSignatureEnum _$hostComputerSignatureEnum_verified =
    const HostComputerSignatureEnum._('verified');
const HostComputerSignatureEnum _$hostComputerSignatureEnum_development =
    const HostComputerSignatureEnum._('development');
const HostComputerSignatureEnum
_$hostComputerSignatureEnum_unknownDefaultOpenApi =
    const HostComputerSignatureEnum._('unknownDefaultOpenApi');

HostComputerSignatureEnum _$hostComputerSignatureEnumValueOf(String name) {
  switch (name) {
    case 'verified':
      return _$hostComputerSignatureEnum_verified;
    case 'development':
      return _$hostComputerSignatureEnum_development;
    case 'unknownDefaultOpenApi':
      return _$hostComputerSignatureEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerSignatureEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerSignatureEnum> _$hostComputerSignatureEnumValues =
    BuiltSet<HostComputerSignatureEnum>(const <HostComputerSignatureEnum>[
      _$hostComputerSignatureEnum_verified,
      _$hostComputerSignatureEnum_development,
      _$hostComputerSignatureEnum_unknownDefaultOpenApi,
    ]);

Serializer<HostComputerHelperEnum> _$hostComputerHelperEnumSerializer =
    _$HostComputerHelperEnumSerializer();
Serializer<HostComputerSignatureEnum> _$hostComputerSignatureEnumSerializer =
    _$HostComputerSignatureEnumSerializer();

class _$HostComputerHelperEnumSerializer
    implements PrimitiveSerializer<HostComputerHelperEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'connected': 'connected',
    'absent': 'absent',
    'refused': 'refused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'connected': 'connected',
    'absent': 'absent',
    'refused': 'refused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerHelperEnum];
  @override
  final String wireName = 'HostComputerHelperEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerHelperEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerHelperEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerHelperEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerSignatureEnumSerializer
    implements PrimitiveSerializer<HostComputerSignatureEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'verified': 'verified',
    'development': 'development',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'verified': 'verified',
    'development': 'development',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerSignatureEnum];
  @override
  final String wireName = 'HostComputerSignatureEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerSignatureEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerSignatureEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerSignatureEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputer extends HostComputer {
  @override
  final HostComputerHelperEnum helper;
  @override
  final bool enabled;
  @override
  final int generation;
  @override
  final String osAccount;
  @override
  final HostComputerPermissions permissions;
  @override
  final String helperVersion;
  @override
  final HostComputerSignatureEnum signature;
  @override
  final bool locked;

  factory _$HostComputer([void Function(HostComputerBuilder)? updates]) =>
      (HostComputerBuilder()..update(updates))._build();

  _$HostComputer._({
    required this.helper,
    required this.enabled,
    required this.generation,
    required this.osAccount,
    required this.permissions,
    required this.helperVersion,
    required this.signature,
    required this.locked,
  }) : super._();
  @override
  HostComputer rebuild(void Function(HostComputerBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerBuilder toBuilder() => HostComputerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputer &&
        helper == other.helper &&
        enabled == other.enabled &&
        generation == other.generation &&
        osAccount == other.osAccount &&
        permissions == other.permissions &&
        helperVersion == other.helperVersion &&
        signature == other.signature &&
        locked == other.locked;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, helper.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, generation.hashCode);
    _$hash = $jc(_$hash, osAccount.hashCode);
    _$hash = $jc(_$hash, permissions.hashCode);
    _$hash = $jc(_$hash, helperVersion.hashCode);
    _$hash = $jc(_$hash, signature.hashCode);
    _$hash = $jc(_$hash, locked.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputer')
          ..add('helper', helper)
          ..add('enabled', enabled)
          ..add('generation', generation)
          ..add('osAccount', osAccount)
          ..add('permissions', permissions)
          ..add('helperVersion', helperVersion)
          ..add('signature', signature)
          ..add('locked', locked))
        .toString();
  }
}

class HostComputerBuilder
    implements Builder<HostComputer, HostComputerBuilder> {
  _$HostComputer? _$v;

  HostComputerHelperEnum? _helper;
  HostComputerHelperEnum? get helper => _$this._helper;
  set helper(HostComputerHelperEnum? helper) => _$this._helper = helper;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  int? _generation;
  int? get generation => _$this._generation;
  set generation(int? generation) => _$this._generation = generation;

  String? _osAccount;
  String? get osAccount => _$this._osAccount;
  set osAccount(String? osAccount) => _$this._osAccount = osAccount;

  HostComputerPermissionsBuilder? _permissions;
  HostComputerPermissionsBuilder get permissions =>
      _$this._permissions ??= HostComputerPermissionsBuilder();
  set permissions(HostComputerPermissionsBuilder? permissions) =>
      _$this._permissions = permissions;

  String? _helperVersion;
  String? get helperVersion => _$this._helperVersion;
  set helperVersion(String? helperVersion) =>
      _$this._helperVersion = helperVersion;

  HostComputerSignatureEnum? _signature;
  HostComputerSignatureEnum? get signature => _$this._signature;
  set signature(HostComputerSignatureEnum? signature) =>
      _$this._signature = signature;

  bool? _locked;
  bool? get locked => _$this._locked;
  set locked(bool? locked) => _$this._locked = locked;

  HostComputerBuilder() {
    HostComputer._defaults(this);
  }

  HostComputerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _helper = $v.helper;
      _enabled = $v.enabled;
      _generation = $v.generation;
      _osAccount = $v.osAccount;
      _permissions = $v.permissions.toBuilder();
      _helperVersion = $v.helperVersion;
      _signature = $v.signature;
      _locked = $v.locked;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputer other) {
    _$v = other as _$HostComputer;
  }

  @override
  void update(void Function(HostComputerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputer build() => _build();

  _$HostComputer _build() {
    _$HostComputer _$result;
    try {
      _$result =
          _$v ??
          _$HostComputer._(
            helper: BuiltValueNullFieldError.checkNotNull(
              helper,
              r'HostComputer',
              'helper',
            ),
            enabled: BuiltValueNullFieldError.checkNotNull(
              enabled,
              r'HostComputer',
              'enabled',
            ),
            generation: BuiltValueNullFieldError.checkNotNull(
              generation,
              r'HostComputer',
              'generation',
            ),
            osAccount: BuiltValueNullFieldError.checkNotNull(
              osAccount,
              r'HostComputer',
              'osAccount',
            ),
            permissions: permissions.build(),
            helperVersion: BuiltValueNullFieldError.checkNotNull(
              helperVersion,
              r'HostComputer',
              'helperVersion',
            ),
            signature: BuiltValueNullFieldError.checkNotNull(
              signature,
              r'HostComputer',
              'signature',
            ),
            locked: BuiltValueNullFieldError.checkNotNull(
              locked,
              r'HostComputer',
              'locked',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'permissions';
        permissions.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostComputer',
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
