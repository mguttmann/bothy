// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_template.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentTemplateSource_Enum _$agentTemplateSourceEnum_shipped =
    const AgentTemplateSource_Enum._('shipped');
const AgentTemplateSource_Enum _$agentTemplateSourceEnum_private =
    const AgentTemplateSource_Enum._('private');
const AgentTemplateSource_Enum _$agentTemplateSourceEnum_unknownDefaultOpenApi =
    const AgentTemplateSource_Enum._('unknownDefaultOpenApi');

AgentTemplateSource_Enum _$agentTemplateSourceEnumValueOf(String name) {
  switch (name) {
    case 'shipped':
      return _$agentTemplateSourceEnum_shipped;
    case 'private':
      return _$agentTemplateSourceEnum_private;
    case 'unknownDefaultOpenApi':
      return _$agentTemplateSourceEnum_unknownDefaultOpenApi;
    default:
      return _$agentTemplateSourceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentTemplateSource_Enum> _$agentTemplateSourceEnumValues =
    BuiltSet<AgentTemplateSource_Enum>(const <AgentTemplateSource_Enum>[
      _$agentTemplateSourceEnum_shipped,
      _$agentTemplateSourceEnum_private,
      _$agentTemplateSourceEnum_unknownDefaultOpenApi,
    ]);

Serializer<AgentTemplateSource_Enum> _$agentTemplateSourceEnumSerializer =
    _$AgentTemplateSource_EnumSerializer();

class _$AgentTemplateSource_EnumSerializer
    implements PrimitiveSerializer<AgentTemplateSource_Enum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'shipped': 'shipped',
    'private': 'private',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'shipped': 'shipped',
    'private': 'private',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentTemplateSource_Enum];
  @override
  final String wireName = 'AgentTemplateSource_Enum';

  @override
  Object serialize(
    Serializers serializers,
    AgentTemplateSource_Enum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentTemplateSource_Enum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentTemplateSource_Enum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentTemplate extends AgentTemplate {
  @override
  final String slug;
  @override
  final AgentTemplateSource_Enum source_;
  @override
  final String name;
  @override
  final String description;
  @override
  final String version;
  @override
  final BuiltList<String> requiredSecrets;
  @override
  final AgentDefinition definition;

  factory _$AgentTemplate([void Function(AgentTemplateBuilder)? updates]) =>
      (AgentTemplateBuilder()..update(updates))._build();

  _$AgentTemplate._({
    required this.slug,
    required this.source_,
    required this.name,
    required this.description,
    required this.version,
    required this.requiredSecrets,
    required this.definition,
  }) : super._();
  @override
  AgentTemplate rebuild(void Function(AgentTemplateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentTemplateBuilder toBuilder() => AgentTemplateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentTemplate &&
        slug == other.slug &&
        source_ == other.source_ &&
        name == other.name &&
        description == other.description &&
        version == other.version &&
        requiredSecrets == other.requiredSecrets &&
        definition == other.definition;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, slug.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, requiredSecrets.hashCode);
    _$hash = $jc(_$hash, definition.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentTemplate')
          ..add('slug', slug)
          ..add('source_', source_)
          ..add('name', name)
          ..add('description', description)
          ..add('version', version)
          ..add('requiredSecrets', requiredSecrets)
          ..add('definition', definition))
        .toString();
  }
}

class AgentTemplateBuilder
    implements Builder<AgentTemplate, AgentTemplateBuilder> {
  _$AgentTemplate? _$v;

  String? _slug;
  String? get slug => _$this._slug;
  set slug(String? slug) => _$this._slug = slug;

  AgentTemplateSource_Enum? _source_;
  AgentTemplateSource_Enum? get source_ => _$this._source_;
  set source_(AgentTemplateSource_Enum? source_) => _$this._source_ = source_;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _version;
  String? get version => _$this._version;
  set version(String? version) => _$this._version = version;

  ListBuilder<String>? _requiredSecrets;
  ListBuilder<String> get requiredSecrets =>
      _$this._requiredSecrets ??= ListBuilder<String>();
  set requiredSecrets(ListBuilder<String>? requiredSecrets) =>
      _$this._requiredSecrets = requiredSecrets;

  AgentDefinitionBuilder? _definition;
  AgentDefinitionBuilder get definition =>
      _$this._definition ??= AgentDefinitionBuilder();
  set definition(AgentDefinitionBuilder? definition) =>
      _$this._definition = definition;

  AgentTemplateBuilder() {
    AgentTemplate._defaults(this);
  }

  AgentTemplateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _slug = $v.slug;
      _source_ = $v.source_;
      _name = $v.name;
      _description = $v.description;
      _version = $v.version;
      _requiredSecrets = $v.requiredSecrets.toBuilder();
      _definition = $v.definition.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentTemplate other) {
    _$v = other as _$AgentTemplate;
  }

  @override
  void update(void Function(AgentTemplateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentTemplate build() => _build();

  _$AgentTemplate _build() {
    _$AgentTemplate _$result;
    try {
      _$result =
          _$v ??
          _$AgentTemplate._(
            slug: BuiltValueNullFieldError.checkNotNull(
              slug,
              r'AgentTemplate',
              'slug',
            ),
            source_: BuiltValueNullFieldError.checkNotNull(
              source_,
              r'AgentTemplate',
              'source_',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AgentTemplate',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'AgentTemplate',
              'description',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'AgentTemplate',
              'version',
            ),
            requiredSecrets: requiredSecrets.build(),
            definition: definition.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'requiredSecrets';
        requiredSecrets.build();
        _$failedField = 'definition';
        definition.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentTemplate',
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
