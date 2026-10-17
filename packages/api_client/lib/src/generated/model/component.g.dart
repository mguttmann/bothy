// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'component.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComponentStateEnum _$componentStateEnum_active =
    const ComponentStateEnum._('active');
const ComponentStateEnum _$componentStateEnum_used = const ComponentStateEnum._(
  'used',
);
const ComponentStateEnum _$componentStateEnum_expired =
    const ComponentStateEnum._('expired');
const ComponentStateEnum _$componentStateEnum_disabled =
    const ComponentStateEnum._('disabled');
const ComponentStateEnum _$componentStateEnum_unknownDefaultOpenApi =
    const ComponentStateEnum._('unknownDefaultOpenApi');

ComponentStateEnum _$componentStateEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$componentStateEnum_active;
    case 'used':
      return _$componentStateEnum_used;
    case 'expired':
      return _$componentStateEnum_expired;
    case 'disabled':
      return _$componentStateEnum_disabled;
    case 'unknownDefaultOpenApi':
      return _$componentStateEnum_unknownDefaultOpenApi;
    default:
      return _$componentStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ComponentStateEnum> _$componentStateEnumValues =
    BuiltSet<ComponentStateEnum>(const <ComponentStateEnum>[
      _$componentStateEnum_active,
      _$componentStateEnum_used,
      _$componentStateEnum_expired,
      _$componentStateEnum_disabled,
      _$componentStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<ComponentStateEnum> _$componentStateEnumSerializer =
    _$ComponentStateEnumSerializer();

class _$ComponentStateEnumSerializer
    implements PrimitiveSerializer<ComponentStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'used': 'used',
    'expired': 'expired',
    'disabled': 'disabled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'used': 'used',
    'expired': 'expired',
    'disabled': 'disabled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComponentStateEnum];
  @override
  final String wireName = 'ComponentStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    ComponentStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComponentStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComponentStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Component extends Component {
  @override
  final String id;
  @override
  final String customId;
  @override
  final ComponentType type;
  @override
  final BuiltMap<String, JsonObject?> definition;
  @override
  final int position;
  @override
  final bool singleUse;
  @override
  final Capability? requiredCapability;
  @override
  final ComponentRestriction restrictedTo;
  @override
  final String? runId;
  @override
  final ComponentStateEnum state;
  @override
  final DateTime? expiresAt;
  @override
  final String? usedBy;
  @override
  final DateTime? usedAt;
  @override
  final int version;

  factory _$Component([void Function(ComponentBuilder)? updates]) =>
      (ComponentBuilder()..update(updates))._build();

  _$Component._({
    required this.id,
    required this.customId,
    required this.type,
    required this.definition,
    required this.position,
    required this.singleUse,
    this.requiredCapability,
    required this.restrictedTo,
    this.runId,
    required this.state,
    this.expiresAt,
    this.usedBy,
    this.usedAt,
    required this.version,
  }) : super._();
  @override
  Component rebuild(void Function(ComponentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ComponentBuilder toBuilder() => ComponentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Component &&
        id == other.id &&
        customId == other.customId &&
        type == other.type &&
        definition == other.definition &&
        position == other.position &&
        singleUse == other.singleUse &&
        requiredCapability == other.requiredCapability &&
        restrictedTo == other.restrictedTo &&
        runId == other.runId &&
        state == other.state &&
        expiresAt == other.expiresAt &&
        usedBy == other.usedBy &&
        usedAt == other.usedAt &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, customId.hashCode);
    _$hash = $jc(_$hash, type.hashCode);
    _$hash = $jc(_$hash, definition.hashCode);
    _$hash = $jc(_$hash, position.hashCode);
    _$hash = $jc(_$hash, singleUse.hashCode);
    _$hash = $jc(_$hash, requiredCapability.hashCode);
    _$hash = $jc(_$hash, restrictedTo.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, usedBy.hashCode);
    _$hash = $jc(_$hash, usedAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Component')
          ..add('id', id)
          ..add('customId', customId)
          ..add('type', type)
          ..add('definition', definition)
          ..add('position', position)
          ..add('singleUse', singleUse)
          ..add('requiredCapability', requiredCapability)
          ..add('restrictedTo', restrictedTo)
          ..add('runId', runId)
          ..add('state', state)
          ..add('expiresAt', expiresAt)
          ..add('usedBy', usedBy)
          ..add('usedAt', usedAt)
          ..add('version', version))
        .toString();
  }
}

class ComponentBuilder implements Builder<Component, ComponentBuilder> {
  _$Component? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _customId;
  String? get customId => _$this._customId;
  set customId(String? customId) => _$this._customId = customId;

  ComponentType? _type;
  ComponentType? get type => _$this._type;
  set type(ComponentType? type) => _$this._type = type;

  MapBuilder<String, JsonObject?>? _definition;
  MapBuilder<String, JsonObject?> get definition =>
      _$this._definition ??= MapBuilder<String, JsonObject?>();
  set definition(MapBuilder<String, JsonObject?>? definition) =>
      _$this._definition = definition;

  int? _position;
  int? get position => _$this._position;
  set position(int? position) => _$this._position = position;

  bool? _singleUse;
  bool? get singleUse => _$this._singleUse;
  set singleUse(bool? singleUse) => _$this._singleUse = singleUse;

  Capability? _requiredCapability;
  Capability? get requiredCapability => _$this._requiredCapability;
  set requiredCapability(Capability? requiredCapability) =>
      _$this._requiredCapability = requiredCapability;

  ComponentRestriction? _restrictedTo;
  ComponentRestriction? get restrictedTo => _$this._restrictedTo;
  set restrictedTo(ComponentRestriction? restrictedTo) =>
      _$this._restrictedTo = restrictedTo;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  ComponentStateEnum? _state;
  ComponentStateEnum? get state => _$this._state;
  set state(ComponentStateEnum? state) => _$this._state = state;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  String? _usedBy;
  String? get usedBy => _$this._usedBy;
  set usedBy(String? usedBy) => _$this._usedBy = usedBy;

  DateTime? _usedAt;
  DateTime? get usedAt => _$this._usedAt;
  set usedAt(DateTime? usedAt) => _$this._usedAt = usedAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  ComponentBuilder() {
    Component._defaults(this);
  }

  ComponentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _customId = $v.customId;
      _type = $v.type;
      _definition = $v.definition.toBuilder();
      _position = $v.position;
      _singleUse = $v.singleUse;
      _requiredCapability = $v.requiredCapability;
      _restrictedTo = $v.restrictedTo;
      _runId = $v.runId;
      _state = $v.state;
      _expiresAt = $v.expiresAt;
      _usedBy = $v.usedBy;
      _usedAt = $v.usedAt;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Component other) {
    _$v = other as _$Component;
  }

  @override
  void update(void Function(ComponentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Component build() => _build();

  _$Component _build() {
    _$Component _$result;
    try {
      _$result =
          _$v ??
          _$Component._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Component', 'id'),
            customId: BuiltValueNullFieldError.checkNotNull(
              customId,
              r'Component',
              'customId',
            ),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'Component',
              'type',
            ),
            definition: definition.build(),
            position: BuiltValueNullFieldError.checkNotNull(
              position,
              r'Component',
              'position',
            ),
            singleUse: BuiltValueNullFieldError.checkNotNull(
              singleUse,
              r'Component',
              'singleUse',
            ),
            requiredCapability: requiredCapability,
            restrictedTo: BuiltValueNullFieldError.checkNotNull(
              restrictedTo,
              r'Component',
              'restrictedTo',
            ),
            runId: runId,
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Component',
              'state',
            ),
            expiresAt: expiresAt,
            usedBy: usedBy,
            usedAt: usedAt,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Component',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'definition';
        definition.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Component',
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
