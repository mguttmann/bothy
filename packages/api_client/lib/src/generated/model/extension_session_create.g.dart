// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'extension_session_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ExtensionSessionCreateClientComponentEnum
_$extensionSessionCreateClientComponentEnum_shareExt =
    const ExtensionSessionCreateClientComponentEnum._('shareExt');
const ExtensionSessionCreateClientComponentEnum
_$extensionSessionCreateClientComponentEnum_unknownDefaultOpenApi =
    const ExtensionSessionCreateClientComponentEnum._('unknownDefaultOpenApi');

ExtensionSessionCreateClientComponentEnum
_$extensionSessionCreateClientComponentEnumValueOf(String name) {
  switch (name) {
    case 'shareExt':
      return _$extensionSessionCreateClientComponentEnum_shareExt;
    case 'unknownDefaultOpenApi':
      return _$extensionSessionCreateClientComponentEnum_unknownDefaultOpenApi;
    default:
      return _$extensionSessionCreateClientComponentEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ExtensionSessionCreateClientComponentEnum>
_$extensionSessionCreateClientComponentEnumValues =
    BuiltSet<ExtensionSessionCreateClientComponentEnum>(
      const <ExtensionSessionCreateClientComponentEnum>[
        _$extensionSessionCreateClientComponentEnum_shareExt,
        _$extensionSessionCreateClientComponentEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ExtensionSessionCreateClientComponentEnum>
_$extensionSessionCreateClientComponentEnumSerializer =
    _$ExtensionSessionCreateClientComponentEnumSerializer();

class _$ExtensionSessionCreateClientComponentEnumSerializer
    implements PrimitiveSerializer<ExtensionSessionCreateClientComponentEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'shareExt': 'share_ext',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'share_ext': 'shareExt',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ExtensionSessionCreateClientComponentEnum,
  ];
  @override
  final String wireName = 'ExtensionSessionCreateClientComponentEnum';

  @override
  Object serialize(
    Serializers serializers,
    ExtensionSessionCreateClientComponentEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ExtensionSessionCreateClientComponentEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ExtensionSessionCreateClientComponentEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ExtensionSessionCreate extends ExtensionSessionCreate {
  @override
  final ExtensionSessionCreateClientComponentEnum clientComponent;

  factory _$ExtensionSessionCreate([
    void Function(ExtensionSessionCreateBuilder)? updates,
  ]) => (ExtensionSessionCreateBuilder()..update(updates))._build();

  _$ExtensionSessionCreate._({required this.clientComponent}) : super._();
  @override
  ExtensionSessionCreate rebuild(
    void Function(ExtensionSessionCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ExtensionSessionCreateBuilder toBuilder() =>
      ExtensionSessionCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ExtensionSessionCreate &&
        clientComponent == other.clientComponent;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, clientComponent.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ExtensionSessionCreate',
    )..add('clientComponent', clientComponent)).toString();
  }
}

class ExtensionSessionCreateBuilder
    implements Builder<ExtensionSessionCreate, ExtensionSessionCreateBuilder> {
  _$ExtensionSessionCreate? _$v;

  ExtensionSessionCreateClientComponentEnum? _clientComponent;
  ExtensionSessionCreateClientComponentEnum? get clientComponent =>
      _$this._clientComponent;
  set clientComponent(
    ExtensionSessionCreateClientComponentEnum? clientComponent,
  ) => _$this._clientComponent = clientComponent;

  ExtensionSessionCreateBuilder() {
    ExtensionSessionCreate._defaults(this);
  }

  ExtensionSessionCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _clientComponent = $v.clientComponent;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ExtensionSessionCreate other) {
    _$v = other as _$ExtensionSessionCreate;
  }

  @override
  void update(void Function(ExtensionSessionCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ExtensionSessionCreate build() => _build();

  _$ExtensionSessionCreate _build() {
    final _$result =
        _$v ??
        _$ExtensionSessionCreate._(
          clientComponent: BuiltValueNullFieldError.checkNotNull(
            clientComponent,
            r'ExtensionSessionCreate',
            'clientComponent',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
