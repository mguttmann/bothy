// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_catalogue_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentInterviewCatalogueEntryToolClassWishEnum
_$agentInterviewCatalogueEntryToolClassWishEnum_chat =
    const AgentInterviewCatalogueEntryToolClassWishEnum._('chat');
const AgentInterviewCatalogueEntryToolClassWishEnum
_$agentInterviewCatalogueEntryToolClassWishEnum_scoped =
    const AgentInterviewCatalogueEntryToolClassWishEnum._('scoped');
const AgentInterviewCatalogueEntryToolClassWishEnum
_$agentInterviewCatalogueEntryToolClassWishEnum_host =
    const AgentInterviewCatalogueEntryToolClassWishEnum._('host');
const AgentInterviewCatalogueEntryToolClassWishEnum
_$agentInterviewCatalogueEntryToolClassWishEnum_unknownDefaultOpenApi =
    const AgentInterviewCatalogueEntryToolClassWishEnum._(
      'unknownDefaultOpenApi',
    );

AgentInterviewCatalogueEntryToolClassWishEnum
_$agentInterviewCatalogueEntryToolClassWishEnumValueOf(String name) {
  switch (name) {
    case 'chat':
      return _$agentInterviewCatalogueEntryToolClassWishEnum_chat;
    case 'scoped':
      return _$agentInterviewCatalogueEntryToolClassWishEnum_scoped;
    case 'host':
      return _$agentInterviewCatalogueEntryToolClassWishEnum_host;
    case 'unknownDefaultOpenApi':
      return _$agentInterviewCatalogueEntryToolClassWishEnum_unknownDefaultOpenApi;
    default:
      return _$agentInterviewCatalogueEntryToolClassWishEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInterviewCatalogueEntryToolClassWishEnum>
_$agentInterviewCatalogueEntryToolClassWishEnumValues =
    BuiltSet<AgentInterviewCatalogueEntryToolClassWishEnum>(
      const <AgentInterviewCatalogueEntryToolClassWishEnum>[
        _$agentInterviewCatalogueEntryToolClassWishEnum_chat,
        _$agentInterviewCatalogueEntryToolClassWishEnum_scoped,
        _$agentInterviewCatalogueEntryToolClassWishEnum_host,
        _$agentInterviewCatalogueEntryToolClassWishEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentInterviewCatalogueEntryToolClassWishEnum>
_$agentInterviewCatalogueEntryToolClassWishEnumSerializer =
    _$AgentInterviewCatalogueEntryToolClassWishEnumSerializer();

class _$AgentInterviewCatalogueEntryToolClassWishEnumSerializer
    implements
        PrimitiveSerializer<AgentInterviewCatalogueEntryToolClassWishEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'chat': 'chat',
    'scoped': 'scoped',
    'host': 'host',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'chat': 'chat',
    'scoped': 'scoped',
    'host': 'host',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentInterviewCatalogueEntryToolClassWishEnum,
  ];
  @override
  final String wireName = 'AgentInterviewCatalogueEntryToolClassWishEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewCatalogueEntryToolClassWishEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInterviewCatalogueEntryToolClassWishEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInterviewCatalogueEntryToolClassWishEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInterviewCatalogueEntry extends AgentInterviewCatalogueEntry {
  @override
  final String slug;
  @override
  final String name;
  @override
  final String description;
  @override
  final AgentInterviewCatalogueEntryToolClassWishEnum toolClassWish;
  @override
  final BuiltList<AgentDefinitionMcpServer> mcpServers;

  factory _$AgentInterviewCatalogueEntry([
    void Function(AgentInterviewCatalogueEntryBuilder)? updates,
  ]) => (AgentInterviewCatalogueEntryBuilder()..update(updates))._build();

  _$AgentInterviewCatalogueEntry._({
    required this.slug,
    required this.name,
    required this.description,
    required this.toolClassWish,
    required this.mcpServers,
  }) : super._();
  @override
  AgentInterviewCatalogueEntry rebuild(
    void Function(AgentInterviewCatalogueEntryBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewCatalogueEntryBuilder toBuilder() =>
      AgentInterviewCatalogueEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewCatalogueEntry &&
        slug == other.slug &&
        name == other.name &&
        description == other.description &&
        toolClassWish == other.toolClassWish &&
        mcpServers == other.mcpServers;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, slug.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, toolClassWish.hashCode);
    _$hash = $jc(_$hash, mcpServers.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewCatalogueEntry')
          ..add('slug', slug)
          ..add('name', name)
          ..add('description', description)
          ..add('toolClassWish', toolClassWish)
          ..add('mcpServers', mcpServers))
        .toString();
  }
}

class AgentInterviewCatalogueEntryBuilder
    implements
        Builder<
          AgentInterviewCatalogueEntry,
          AgentInterviewCatalogueEntryBuilder
        > {
  _$AgentInterviewCatalogueEntry? _$v;

  String? _slug;
  String? get slug => _$this._slug;
  set slug(String? slug) => _$this._slug = slug;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  AgentInterviewCatalogueEntryToolClassWishEnum? _toolClassWish;
  AgentInterviewCatalogueEntryToolClassWishEnum? get toolClassWish =>
      _$this._toolClassWish;
  set toolClassWish(
    AgentInterviewCatalogueEntryToolClassWishEnum? toolClassWish,
  ) => _$this._toolClassWish = toolClassWish;

  ListBuilder<AgentDefinitionMcpServer>? _mcpServers;
  ListBuilder<AgentDefinitionMcpServer> get mcpServers =>
      _$this._mcpServers ??= ListBuilder<AgentDefinitionMcpServer>();
  set mcpServers(ListBuilder<AgentDefinitionMcpServer>? mcpServers) =>
      _$this._mcpServers = mcpServers;

  AgentInterviewCatalogueEntryBuilder() {
    AgentInterviewCatalogueEntry._defaults(this);
  }

  AgentInterviewCatalogueEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _slug = $v.slug;
      _name = $v.name;
      _description = $v.description;
      _toolClassWish = $v.toolClassWish;
      _mcpServers = $v.mcpServers.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewCatalogueEntry other) {
    _$v = other as _$AgentInterviewCatalogueEntry;
  }

  @override
  void update(void Function(AgentInterviewCatalogueEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewCatalogueEntry build() => _build();

  _$AgentInterviewCatalogueEntry _build() {
    _$AgentInterviewCatalogueEntry _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewCatalogueEntry._(
            slug: BuiltValueNullFieldError.checkNotNull(
              slug,
              r'AgentInterviewCatalogueEntry',
              'slug',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AgentInterviewCatalogueEntry',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'AgentInterviewCatalogueEntry',
              'description',
            ),
            toolClassWish: BuiltValueNullFieldError.checkNotNull(
              toolClassWish,
              r'AgentInterviewCatalogueEntry',
              'toolClassWish',
            ),
            mcpServers: mcpServers.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'mcpServers';
        mcpServers.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewCatalogueEntry',
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
