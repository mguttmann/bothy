// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'component_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComponentUpdateStateEnum _$componentUpdateStateEnum_active =
    const ComponentUpdateStateEnum._('active');
const ComponentUpdateStateEnum _$componentUpdateStateEnum_disabled =
    const ComponentUpdateStateEnum._('disabled');
const ComponentUpdateStateEnum
_$componentUpdateStateEnum_unknownDefaultOpenApi =
    const ComponentUpdateStateEnum._('unknownDefaultOpenApi');

ComponentUpdateStateEnum _$componentUpdateStateEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$componentUpdateStateEnum_active;
    case 'disabled':
      return _$componentUpdateStateEnum_disabled;
    case 'unknownDefaultOpenApi':
      return _$componentUpdateStateEnum_unknownDefaultOpenApi;
    default:
      return _$componentUpdateStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ComponentUpdateStateEnum> _$componentUpdateStateEnumValues =
    BuiltSet<ComponentUpdateStateEnum>(const <ComponentUpdateStateEnum>[
      _$componentUpdateStateEnum_active,
      _$componentUpdateStateEnum_disabled,
      _$componentUpdateStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<ComponentUpdateStateEnum> _$componentUpdateStateEnumSerializer =
    _$ComponentUpdateStateEnumSerializer();

class _$ComponentUpdateStateEnumSerializer
    implements PrimitiveSerializer<ComponentUpdateStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'disabled': 'disabled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'disabled': 'disabled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComponentUpdateStateEnum];
  @override
  final String wireName = 'ComponentUpdateStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    ComponentUpdateStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComponentUpdateStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComponentUpdateStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ComponentUpdate extends ComponentUpdate {
  @override
  final ComponentUpdateStateEnum? state;
  @override
  final BuiltMap<String, JsonObject?>? definition;

  factory _$ComponentUpdate([void Function(ComponentUpdateBuilder)? updates]) =>
      (ComponentUpdateBuilder()..update(updates))._build();

  _$ComponentUpdate._({this.state, this.definition}) : super._();
  @override
  ComponentUpdate rebuild(void Function(ComponentUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ComponentUpdateBuilder toBuilder() => ComponentUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComponentUpdate &&
        state == other.state &&
        definition == other.definition;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, definition.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComponentUpdate')
          ..add('state', state)
          ..add('definition', definition))
        .toString();
  }
}

class ComponentUpdateBuilder
    implements Builder<ComponentUpdate, ComponentUpdateBuilder> {
  _$ComponentUpdate? _$v;

  ComponentUpdateStateEnum? _state;
  ComponentUpdateStateEnum? get state => _$this._state;
  set state(ComponentUpdateStateEnum? state) => _$this._state = state;

  MapBuilder<String, JsonObject?>? _definition;
  MapBuilder<String, JsonObject?> get definition =>
      _$this._definition ??= MapBuilder<String, JsonObject?>();
  set definition(MapBuilder<String, JsonObject?>? definition) =>
      _$this._definition = definition;

  ComponentUpdateBuilder() {
    ComponentUpdate._defaults(this);
  }

  ComponentUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _definition = $v.definition?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComponentUpdate other) {
    _$v = other as _$ComponentUpdate;
  }

  @override
  void update(void Function(ComponentUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComponentUpdate build() => _build();

  _$ComponentUpdate _build() {
    _$ComponentUpdate _$result;
    try {
      _$result =
          _$v ??
          _$ComponentUpdate._(state: state, definition: _definition?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'definition';
        _definition?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ComponentUpdate',
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
