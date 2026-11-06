// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_version.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentDefinitionVersionOriginKindEnum
_$agentDefinitionVersionOriginKindEnum_template =
    const AgentDefinitionVersionOriginKindEnum._('template');
const AgentDefinitionVersionOriginKindEnum
_$agentDefinitionVersionOriginKindEnum_interview =
    const AgentDefinitionVersionOriginKindEnum._('interview');
const AgentDefinitionVersionOriginKindEnum
_$agentDefinitionVersionOriginKindEnum_manual =
    const AgentDefinitionVersionOriginKindEnum._('manual');
const AgentDefinitionVersionOriginKindEnum
_$agentDefinitionVersionOriginKindEnum_unknownDefaultOpenApi =
    const AgentDefinitionVersionOriginKindEnum._('unknownDefaultOpenApi');

AgentDefinitionVersionOriginKindEnum
_$agentDefinitionVersionOriginKindEnumValueOf(String name) {
  switch (name) {
    case 'template':
      return _$agentDefinitionVersionOriginKindEnum_template;
    case 'interview':
      return _$agentDefinitionVersionOriginKindEnum_interview;
    case 'manual':
      return _$agentDefinitionVersionOriginKindEnum_manual;
    case 'unknownDefaultOpenApi':
      return _$agentDefinitionVersionOriginKindEnum_unknownDefaultOpenApi;
    default:
      return _$agentDefinitionVersionOriginKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentDefinitionVersionOriginKindEnum>
_$agentDefinitionVersionOriginKindEnumValues =
    BuiltSet<AgentDefinitionVersionOriginKindEnum>(
      const <AgentDefinitionVersionOriginKindEnum>[
        _$agentDefinitionVersionOriginKindEnum_template,
        _$agentDefinitionVersionOriginKindEnum_interview,
        _$agentDefinitionVersionOriginKindEnum_manual,
        _$agentDefinitionVersionOriginKindEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentDefinitionVersionOriginKindEnum>
_$agentDefinitionVersionOriginKindEnumSerializer =
    _$AgentDefinitionVersionOriginKindEnumSerializer();

class _$AgentDefinitionVersionOriginKindEnumSerializer
    implements PrimitiveSerializer<AgentDefinitionVersionOriginKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'template': 'template',
    'interview': 'interview',
    'manual': 'manual',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'template': 'template',
    'interview': 'interview',
    'manual': 'manual',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentDefinitionVersionOriginKindEnum,
  ];
  @override
  final String wireName = 'AgentDefinitionVersionOriginKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionVersionOriginKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentDefinitionVersionOriginKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentDefinitionVersionOriginKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentDefinitionVersion extends AgentDefinitionVersion {
  @override
  final String agentId;
  @override
  final int version;
  @override
  final AgentDefinitionVersionOriginKindEnum originKind;
  @override
  final DateTime createdAt;
  @override
  final String createdBy;
  @override
  final AgentDefinition? definition;

  factory _$AgentDefinitionVersion([
    void Function(AgentDefinitionVersionBuilder)? updates,
  ]) => (AgentDefinitionVersionBuilder()..update(updates))._build();

  _$AgentDefinitionVersion._({
    required this.agentId,
    required this.version,
    required this.originKind,
    required this.createdAt,
    required this.createdBy,
    this.definition,
  }) : super._();
  @override
  AgentDefinitionVersion rebuild(
    void Function(AgentDefinitionVersionBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionVersionBuilder toBuilder() =>
      AgentDefinitionVersionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionVersion &&
        agentId == other.agentId &&
        version == other.version &&
        originKind == other.originKind &&
        createdAt == other.createdAt &&
        createdBy == other.createdBy &&
        definition == other.definition;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, originKind.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, createdBy.hashCode);
    _$hash = $jc(_$hash, definition.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionVersion')
          ..add('agentId', agentId)
          ..add('version', version)
          ..add('originKind', originKind)
          ..add('createdAt', createdAt)
          ..add('createdBy', createdBy)
          ..add('definition', definition))
        .toString();
  }
}

class AgentDefinitionVersionBuilder
    implements Builder<AgentDefinitionVersion, AgentDefinitionVersionBuilder> {
  _$AgentDefinitionVersion? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  AgentDefinitionVersionOriginKindEnum? _originKind;
  AgentDefinitionVersionOriginKindEnum? get originKind => _$this._originKind;
  set originKind(AgentDefinitionVersionOriginKindEnum? originKind) =>
      _$this._originKind = originKind;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  String? _createdBy;
  String? get createdBy => _$this._createdBy;
  set createdBy(String? createdBy) => _$this._createdBy = createdBy;

  AgentDefinitionBuilder? _definition;
  AgentDefinitionBuilder get definition =>
      _$this._definition ??= AgentDefinitionBuilder();
  set definition(AgentDefinitionBuilder? definition) =>
      _$this._definition = definition;

  AgentDefinitionVersionBuilder() {
    AgentDefinitionVersion._defaults(this);
  }

  AgentDefinitionVersionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _version = $v.version;
      _originKind = $v.originKind;
      _createdAt = $v.createdAt;
      _createdBy = $v.createdBy;
      _definition = $v.definition?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionVersion other) {
    _$v = other as _$AgentDefinitionVersion;
  }

  @override
  void update(void Function(AgentDefinitionVersionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionVersion build() => _build();

  _$AgentDefinitionVersion _build() {
    _$AgentDefinitionVersion _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinitionVersion._(
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'AgentDefinitionVersion',
              'agentId',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'AgentDefinitionVersion',
              'version',
            ),
            originKind: BuiltValueNullFieldError.checkNotNull(
              originKind,
              r'AgentDefinitionVersion',
              'originKind',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'AgentDefinitionVersion',
              'createdAt',
            ),
            createdBy: BuiltValueNullFieldError.checkNotNull(
              createdBy,
              r'AgentDefinitionVersion',
              'createdBy',
            ),
            definition: _definition?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'definition';
        _definition?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinitionVersion',
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
