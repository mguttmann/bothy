// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_permissions.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostComputerPermissionsCaptureEnum
_$hostComputerPermissionsCaptureEnum_granted =
    const HostComputerPermissionsCaptureEnum._('granted');
const HostComputerPermissionsCaptureEnum
_$hostComputerPermissionsCaptureEnum_denied =
    const HostComputerPermissionsCaptureEnum._('denied');
const HostComputerPermissionsCaptureEnum
_$hostComputerPermissionsCaptureEnum_unknown =
    const HostComputerPermissionsCaptureEnum._('unknown');
const HostComputerPermissionsCaptureEnum
_$hostComputerPermissionsCaptureEnum_unknownDefaultOpenApi =
    const HostComputerPermissionsCaptureEnum._('unknownDefaultOpenApi');

HostComputerPermissionsCaptureEnum _$hostComputerPermissionsCaptureEnumValueOf(
  String name,
) {
  switch (name) {
    case 'granted':
      return _$hostComputerPermissionsCaptureEnum_granted;
    case 'denied':
      return _$hostComputerPermissionsCaptureEnum_denied;
    case 'unknown':
      return _$hostComputerPermissionsCaptureEnum_unknown;
    case 'unknownDefaultOpenApi':
      return _$hostComputerPermissionsCaptureEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerPermissionsCaptureEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerPermissionsCaptureEnum>
_$hostComputerPermissionsCaptureEnumValues =
    BuiltSet<HostComputerPermissionsCaptureEnum>(
      const <HostComputerPermissionsCaptureEnum>[
        _$hostComputerPermissionsCaptureEnum_granted,
        _$hostComputerPermissionsCaptureEnum_denied,
        _$hostComputerPermissionsCaptureEnum_unknown,
        _$hostComputerPermissionsCaptureEnum_unknownDefaultOpenApi,
      ],
    );

const HostComputerPermissionsInputEnum
_$hostComputerPermissionsInputEnum_granted =
    const HostComputerPermissionsInputEnum._('granted');
const HostComputerPermissionsInputEnum
_$hostComputerPermissionsInputEnum_denied =
    const HostComputerPermissionsInputEnum._('denied');
const HostComputerPermissionsInputEnum
_$hostComputerPermissionsInputEnum_unknown =
    const HostComputerPermissionsInputEnum._('unknown');
const HostComputerPermissionsInputEnum
_$hostComputerPermissionsInputEnum_unknownDefaultOpenApi =
    const HostComputerPermissionsInputEnum._('unknownDefaultOpenApi');

HostComputerPermissionsInputEnum _$hostComputerPermissionsInputEnumValueOf(
  String name,
) {
  switch (name) {
    case 'granted':
      return _$hostComputerPermissionsInputEnum_granted;
    case 'denied':
      return _$hostComputerPermissionsInputEnum_denied;
    case 'unknown':
      return _$hostComputerPermissionsInputEnum_unknown;
    case 'unknownDefaultOpenApi':
      return _$hostComputerPermissionsInputEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerPermissionsInputEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerPermissionsInputEnum>
_$hostComputerPermissionsInputEnumValues =
    BuiltSet<HostComputerPermissionsInputEnum>(
      const <HostComputerPermissionsInputEnum>[
        _$hostComputerPermissionsInputEnum_granted,
        _$hostComputerPermissionsInputEnum_denied,
        _$hostComputerPermissionsInputEnum_unknown,
        _$hostComputerPermissionsInputEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<HostComputerPermissionsCaptureEnum>
_$hostComputerPermissionsCaptureEnumSerializer =
    _$HostComputerPermissionsCaptureEnumSerializer();
Serializer<HostComputerPermissionsInputEnum>
_$hostComputerPermissionsInputEnumSerializer =
    _$HostComputerPermissionsInputEnumSerializer();

class _$HostComputerPermissionsCaptureEnumSerializer
    implements PrimitiveSerializer<HostComputerPermissionsCaptureEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'granted': 'granted',
    'denied': 'denied',
    'unknown': 'unknown',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'granted': 'granted',
    'denied': 'denied',
    'unknown': 'unknown',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerPermissionsCaptureEnum];
  @override
  final String wireName = 'HostComputerPermissionsCaptureEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerPermissionsCaptureEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerPermissionsCaptureEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerPermissionsCaptureEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerPermissionsInputEnumSerializer
    implements PrimitiveSerializer<HostComputerPermissionsInputEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'granted': 'granted',
    'denied': 'denied',
    'unknown': 'unknown',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'granted': 'granted',
    'denied': 'denied',
    'unknown': 'unknown',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerPermissionsInputEnum];
  @override
  final String wireName = 'HostComputerPermissionsInputEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerPermissionsInputEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerPermissionsInputEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerPermissionsInputEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerPermissions extends HostComputerPermissions {
  @override
  final HostComputerPermissionsCaptureEnum capture;
  @override
  final HostComputerPermissionsInputEnum input;

  factory _$HostComputerPermissions([
    void Function(HostComputerPermissionsBuilder)? updates,
  ]) => (HostComputerPermissionsBuilder()..update(updates))._build();

  _$HostComputerPermissions._({required this.capture, required this.input})
    : super._();
  @override
  HostComputerPermissions rebuild(
    void Function(HostComputerPermissionsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostComputerPermissionsBuilder toBuilder() =>
      HostComputerPermissionsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerPermissions &&
        capture == other.capture &&
        input == other.input;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, capture.hashCode);
    _$hash = $jc(_$hash, input.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerPermissions')
          ..add('capture', capture)
          ..add('input', input))
        .toString();
  }
}

class HostComputerPermissionsBuilder
    implements
        Builder<HostComputerPermissions, HostComputerPermissionsBuilder> {
  _$HostComputerPermissions? _$v;

  HostComputerPermissionsCaptureEnum? _capture;
  HostComputerPermissionsCaptureEnum? get capture => _$this._capture;
  set capture(HostComputerPermissionsCaptureEnum? capture) =>
      _$this._capture = capture;

  HostComputerPermissionsInputEnum? _input;
  HostComputerPermissionsInputEnum? get input => _$this._input;
  set input(HostComputerPermissionsInputEnum? input) => _$this._input = input;

  HostComputerPermissionsBuilder() {
    HostComputerPermissions._defaults(this);
  }

  HostComputerPermissionsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _capture = $v.capture;
      _input = $v.input;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerPermissions other) {
    _$v = other as _$HostComputerPermissions;
  }

  @override
  void update(void Function(HostComputerPermissionsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerPermissions build() => _build();

  _$HostComputerPermissions _build() {
    final _$result =
        _$v ??
        _$HostComputerPermissions._(
          capture: BuiltValueNullFieldError.checkNotNull(
            capture,
            r'HostComputerPermissions',
            'capture',
          ),
          input: BuiltValueNullFieldError.checkNotNull(
            input,
            r'HostComputerPermissions',
            'input',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
