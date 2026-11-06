// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_origin.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentDefinitionOriginKindEnum _$agentDefinitionOriginKindEnum_template =
    const AgentDefinitionOriginKindEnum._('template');
const AgentDefinitionOriginKindEnum _$agentDefinitionOriginKindEnum_interview =
    const AgentDefinitionOriginKindEnum._('interview');
const AgentDefinitionOriginKindEnum _$agentDefinitionOriginKindEnum_manual =
    const AgentDefinitionOriginKindEnum._('manual');
const AgentDefinitionOriginKindEnum
_$agentDefinitionOriginKindEnum_unknownDefaultOpenApi =
    const AgentDefinitionOriginKindEnum._('unknownDefaultOpenApi');

AgentDefinitionOriginKindEnum _$agentDefinitionOriginKindEnumValueOf(
  String name,
) {
  switch (name) {
    case 'template':
      return _$agentDefinitionOriginKindEnum_template;
    case 'interview':
      return _$agentDefinitionOriginKindEnum_interview;
    case 'manual':
      return _$agentDefinitionOriginKindEnum_manual;
    case 'unknownDefaultOpenApi':
      return _$agentDefinitionOriginKindEnum_unknownDefaultOpenApi;
    default:
      return _$agentDefinitionOriginKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentDefinitionOriginKindEnum>
_$agentDefinitionOriginKindEnumValues = BuiltSet<AgentDefinitionOriginKindEnum>(
  const <AgentDefinitionOriginKindEnum>[
    _$agentDefinitionOriginKindEnum_template,
    _$agentDefinitionOriginKindEnum_interview,
    _$agentDefinitionOriginKindEnum_manual,
    _$agentDefinitionOriginKindEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AgentDefinitionOriginKindEnum>
_$agentDefinitionOriginKindEnumSerializer =
    _$AgentDefinitionOriginKindEnumSerializer();

class _$AgentDefinitionOriginKindEnumSerializer
    implements PrimitiveSerializer<AgentDefinitionOriginKindEnum> {
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
  final Iterable<Type> types = const <Type>[AgentDefinitionOriginKindEnum];
  @override
  final String wireName = 'AgentDefinitionOriginKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionOriginKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentDefinitionOriginKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentDefinitionOriginKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentDefinitionOrigin extends AgentDefinitionOrigin {
  @override
  final AgentDefinitionOriginKindEnum kind;
  @override
  final String? templateSlug;
  @override
  final String? templateVersion;

  factory _$AgentDefinitionOrigin([
    void Function(AgentDefinitionOriginBuilder)? updates,
  ]) => (AgentDefinitionOriginBuilder()..update(updates))._build();

  _$AgentDefinitionOrigin._({
    required this.kind,
    this.templateSlug,
    this.templateVersion,
  }) : super._();
  @override
  AgentDefinitionOrigin rebuild(
    void Function(AgentDefinitionOriginBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionOriginBuilder toBuilder() =>
      AgentDefinitionOriginBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionOrigin &&
        kind == other.kind &&
        templateSlug == other.templateSlug &&
        templateVersion == other.templateVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, templateSlug.hashCode);
    _$hash = $jc(_$hash, templateVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionOrigin')
          ..add('kind', kind)
          ..add('templateSlug', templateSlug)
          ..add('templateVersion', templateVersion))
        .toString();
  }
}

class AgentDefinitionOriginBuilder
    implements Builder<AgentDefinitionOrigin, AgentDefinitionOriginBuilder> {
  _$AgentDefinitionOrigin? _$v;

  AgentDefinitionOriginKindEnum? _kind;
  AgentDefinitionOriginKindEnum? get kind => _$this._kind;
  set kind(AgentDefinitionOriginKindEnum? kind) => _$this._kind = kind;

  String? _templateSlug;
  String? get templateSlug => _$this._templateSlug;
  set templateSlug(String? templateSlug) => _$this._templateSlug = templateSlug;

  String? _templateVersion;
  String? get templateVersion => _$this._templateVersion;
  set templateVersion(String? templateVersion) =>
      _$this._templateVersion = templateVersion;

  AgentDefinitionOriginBuilder() {
    AgentDefinitionOrigin._defaults(this);
  }

  AgentDefinitionOriginBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _templateSlug = $v.templateSlug;
      _templateVersion = $v.templateVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionOrigin other) {
    _$v = other as _$AgentDefinitionOrigin;
  }

  @override
  void update(void Function(AgentDefinitionOriginBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionOrigin build() => _build();

  _$AgentDefinitionOrigin _build() {
    final _$result =
        _$v ??
        _$AgentDefinitionOrigin._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'AgentDefinitionOrigin',
            'kind',
          ),
          templateSlug: templateSlug,
          templateVersion: templateVersion,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
