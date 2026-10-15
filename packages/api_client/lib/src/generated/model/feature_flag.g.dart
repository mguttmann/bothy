// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'feature_flag.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const FeatureFlagKindEnum _$featureFlagKindEnum_release =
    const FeatureFlagKindEnum._('release');
const FeatureFlagKindEnum _$featureFlagKindEnum_option =
    const FeatureFlagKindEnum._('option');
const FeatureFlagKindEnum _$featureFlagKindEnum_unknownDefaultOpenApi =
    const FeatureFlagKindEnum._('unknownDefaultOpenApi');

FeatureFlagKindEnum _$featureFlagKindEnumValueOf(String name) {
  switch (name) {
    case 'release':
      return _$featureFlagKindEnum_release;
    case 'option':
      return _$featureFlagKindEnum_option;
    case 'unknownDefaultOpenApi':
      return _$featureFlagKindEnum_unknownDefaultOpenApi;
    default:
      return _$featureFlagKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<FeatureFlagKindEnum> _$featureFlagKindEnumValues =
    BuiltSet<FeatureFlagKindEnum>(const <FeatureFlagKindEnum>[
      _$featureFlagKindEnum_release,
      _$featureFlagKindEnum_option,
      _$featureFlagKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<FeatureFlagKindEnum> _$featureFlagKindEnumSerializer =
    _$FeatureFlagKindEnumSerializer();

class _$FeatureFlagKindEnumSerializer
    implements PrimitiveSerializer<FeatureFlagKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'release': 'release',
    'option': 'option',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'release': 'release',
    'option': 'option',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[FeatureFlagKindEnum];
  @override
  final String wireName = 'FeatureFlagKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    FeatureFlagKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  FeatureFlagKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => FeatureFlagKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$FeatureFlag extends FeatureFlag {
  @override
  final String key;
  @override
  final FeatureFlagKindEnum? kind;
  @override
  final String? description;
  @override
  final bool? default_;
  @override
  final String? removeBy;
  @override
  final String? module;
  @override
  final bool orphaned;
  @override
  final bool enabled;
  @override
  final FlagRollout rollout;
  @override
  final int version;
  @override
  final String? updatedBy;
  @override
  final DateTime? updatedAt;

  factory _$FeatureFlag([void Function(FeatureFlagBuilder)? updates]) =>
      (FeatureFlagBuilder()..update(updates))._build();

  _$FeatureFlag._({
    required this.key,
    this.kind,
    this.description,
    this.default_,
    this.removeBy,
    this.module,
    required this.orphaned,
    required this.enabled,
    required this.rollout,
    required this.version,
    this.updatedBy,
    this.updatedAt,
  }) : super._();
  @override
  FeatureFlag rebuild(void Function(FeatureFlagBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FeatureFlagBuilder toBuilder() => FeatureFlagBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FeatureFlag &&
        key == other.key &&
        kind == other.kind &&
        description == other.description &&
        default_ == other.default_ &&
        removeBy == other.removeBy &&
        module == other.module &&
        orphaned == other.orphaned &&
        enabled == other.enabled &&
        rollout == other.rollout &&
        version == other.version &&
        updatedBy == other.updatedBy &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, key.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, default_.hashCode);
    _$hash = $jc(_$hash, removeBy.hashCode);
    _$hash = $jc(_$hash, module.hashCode);
    _$hash = $jc(_$hash, orphaned.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, rollout.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedBy.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'FeatureFlag')
          ..add('key', key)
          ..add('kind', kind)
          ..add('description', description)
          ..add('default_', default_)
          ..add('removeBy', removeBy)
          ..add('module', module)
          ..add('orphaned', orphaned)
          ..add('enabled', enabled)
          ..add('rollout', rollout)
          ..add('version', version)
          ..add('updatedBy', updatedBy)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class FeatureFlagBuilder implements Builder<FeatureFlag, FeatureFlagBuilder> {
  _$FeatureFlag? _$v;

  String? _key;
  String? get key => _$this._key;
  set key(String? key) => _$this._key = key;

  FeatureFlagKindEnum? _kind;
  FeatureFlagKindEnum? get kind => _$this._kind;
  set kind(FeatureFlagKindEnum? kind) => _$this._kind = kind;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  bool? _default_;
  bool? get default_ => _$this._default_;
  set default_(bool? default_) => _$this._default_ = default_;

  String? _removeBy;
  String? get removeBy => _$this._removeBy;
  set removeBy(String? removeBy) => _$this._removeBy = removeBy;

  String? _module;
  String? get module => _$this._module;
  set module(String? module) => _$this._module = module;

  bool? _orphaned;
  bool? get orphaned => _$this._orphaned;
  set orphaned(bool? orphaned) => _$this._orphaned = orphaned;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  FlagRolloutBuilder? _rollout;
  FlagRolloutBuilder get rollout => _$this._rollout ??= FlagRolloutBuilder();
  set rollout(FlagRolloutBuilder? rollout) => _$this._rollout = rollout;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedBy;
  String? get updatedBy => _$this._updatedBy;
  set updatedBy(String? updatedBy) => _$this._updatedBy = updatedBy;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  FeatureFlagBuilder() {
    FeatureFlag._defaults(this);
  }

  FeatureFlagBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _key = $v.key;
      _kind = $v.kind;
      _description = $v.description;
      _default_ = $v.default_;
      _removeBy = $v.removeBy;
      _module = $v.module;
      _orphaned = $v.orphaned;
      _enabled = $v.enabled;
      _rollout = $v.rollout.toBuilder();
      _version = $v.version;
      _updatedBy = $v.updatedBy;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FeatureFlag other) {
    _$v = other as _$FeatureFlag;
  }

  @override
  void update(void Function(FeatureFlagBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FeatureFlag build() => _build();

  _$FeatureFlag _build() {
    _$FeatureFlag _$result;
    try {
      _$result =
          _$v ??
          _$FeatureFlag._(
            key: BuiltValueNullFieldError.checkNotNull(
              key,
              r'FeatureFlag',
              'key',
            ),
            kind: kind,
            description: description,
            default_: default_,
            removeBy: removeBy,
            module: module,
            orphaned: BuiltValueNullFieldError.checkNotNull(
              orphaned,
              r'FeatureFlag',
              'orphaned',
            ),
            enabled: BuiltValueNullFieldError.checkNotNull(
              enabled,
              r'FeatureFlag',
              'enabled',
            ),
            rollout: rollout.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'FeatureFlag',
              'version',
            ),
            updatedBy: updatedBy,
            updatedAt: updatedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'rollout';
        rollout.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'FeatureFlag',
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
