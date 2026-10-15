// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'component_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComponentCreate extends ComponentCreate {
  @override
  final String customId;
  @override
  final ComponentType type;
  @override
  final BuiltMap<String, JsonObject?> definition;
  @override
  final bool? singleUse;
  @override
  final Capability? requiredCapability;
  @override
  final ComponentRestriction? restrictedTo;
  @override
  final DateTime? expiresAt;

  factory _$ComponentCreate([void Function(ComponentCreateBuilder)? updates]) =>
      (ComponentCreateBuilder()..update(updates))._build();

  _$ComponentCreate._({
    required this.customId,
    required this.type,
    required this.definition,
    this.singleUse,
    this.requiredCapability,
    this.restrictedTo,
    this.expiresAt,
  }) : super._();
  @override
  ComponentCreate rebuild(void Function(ComponentCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ComponentCreateBuilder toBuilder() => ComponentCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComponentCreate &&
        customId == other.customId &&
        type == other.type &&
        definition == other.definition &&
        singleUse == other.singleUse &&
        requiredCapability == other.requiredCapability &&
        restrictedTo == other.restrictedTo &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, customId.hashCode);
    _$hash = $jc(_$hash, type.hashCode);
    _$hash = $jc(_$hash, definition.hashCode);
    _$hash = $jc(_$hash, singleUse.hashCode);
    _$hash = $jc(_$hash, requiredCapability.hashCode);
    _$hash = $jc(_$hash, restrictedTo.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComponentCreate')
          ..add('customId', customId)
          ..add('type', type)
          ..add('definition', definition)
          ..add('singleUse', singleUse)
          ..add('requiredCapability', requiredCapability)
          ..add('restrictedTo', restrictedTo)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class ComponentCreateBuilder
    implements Builder<ComponentCreate, ComponentCreateBuilder> {
  _$ComponentCreate? _$v;

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

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  ComponentCreateBuilder() {
    ComponentCreate._defaults(this);
  }

  ComponentCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _customId = $v.customId;
      _type = $v.type;
      _definition = $v.definition.toBuilder();
      _singleUse = $v.singleUse;
      _requiredCapability = $v.requiredCapability;
      _restrictedTo = $v.restrictedTo;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComponentCreate other) {
    _$v = other as _$ComponentCreate;
  }

  @override
  void update(void Function(ComponentCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComponentCreate build() => _build();

  _$ComponentCreate _build() {
    _$ComponentCreate _$result;
    try {
      _$result =
          _$v ??
          _$ComponentCreate._(
            customId: BuiltValueNullFieldError.checkNotNull(
              customId,
              r'ComponentCreate',
              'customId',
            ),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'ComponentCreate',
              'type',
            ),
            definition: definition.build(),
            singleUse: singleUse,
            requiredCapability: requiredCapability,
            restrictedTo: restrictedTo,
            expiresAt: expiresAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'definition';
        definition.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ComponentCreate',
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
