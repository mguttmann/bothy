// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_scope.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsScopeKindEnum _$statsScopeKindEnum_instance =
    const StatsScopeKindEnum._('instance');
const StatsScopeKindEnum _$statsScopeKindEnum_space =
    const StatsScopeKindEnum._('space');
const StatsScopeKindEnum _$statsScopeKindEnum_unknownDefaultOpenApi =
    const StatsScopeKindEnum._('unknownDefaultOpenApi');

StatsScopeKindEnum _$statsScopeKindEnumValueOf(String name) {
  switch (name) {
    case 'instance':
      return _$statsScopeKindEnum_instance;
    case 'space':
      return _$statsScopeKindEnum_space;
    case 'unknownDefaultOpenApi':
      return _$statsScopeKindEnum_unknownDefaultOpenApi;
    default:
      return _$statsScopeKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsScopeKindEnum> _$statsScopeKindEnumValues =
    BuiltSet<StatsScopeKindEnum>(const <StatsScopeKindEnum>[
      _$statsScopeKindEnum_instance,
      _$statsScopeKindEnum_space,
      _$statsScopeKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<StatsScopeKindEnum> _$statsScopeKindEnumSerializer =
    _$StatsScopeKindEnumSerializer();

class _$StatsScopeKindEnumSerializer
    implements PrimitiveSerializer<StatsScopeKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'instance': 'instance',
    'space': 'space',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'instance': 'instance',
    'space': 'space',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsScopeKindEnum];
  @override
  final String wireName = 'StatsScopeKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsScopeKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsScopeKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsScopeKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsScope extends StatsScope {
  @override
  final StatsScopeKindEnum kind;
  @override
  final String? spaceId;

  factory _$StatsScope([void Function(StatsScopeBuilder)? updates]) =>
      (StatsScopeBuilder()..update(updates))._build();

  _$StatsScope._({required this.kind, this.spaceId}) : super._();
  @override
  StatsScope rebuild(void Function(StatsScopeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsScopeBuilder toBuilder() => StatsScopeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsScope &&
        kind == other.kind &&
        spaceId == other.spaceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsScope')
          ..add('kind', kind)
          ..add('spaceId', spaceId))
        .toString();
  }
}

class StatsScopeBuilder implements Builder<StatsScope, StatsScopeBuilder> {
  _$StatsScope? _$v;

  StatsScopeKindEnum? _kind;
  StatsScopeKindEnum? get kind => _$this._kind;
  set kind(StatsScopeKindEnum? kind) => _$this._kind = kind;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  StatsScopeBuilder() {
    StatsScope._defaults(this);
  }

  StatsScopeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _spaceId = $v.spaceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsScope other) {
    _$v = other as _$StatsScope;
  }

  @override
  void update(void Function(StatsScopeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsScope build() => _build();

  _$StatsScope _build() {
    final _$result =
        _$v ??
        _$StatsScope._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'StatsScope',
            'kind',
          ),
          spaceId: spaceId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
