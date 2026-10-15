// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'template.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TemplateScopeEnum _$templateScopeEnum_personal =
    const TemplateScopeEnum._('personal');
const TemplateScopeEnum _$templateScopeEnum_channel = const TemplateScopeEnum._(
  'channel',
);
const TemplateScopeEnum _$templateScopeEnum_unknownDefaultOpenApi =
    const TemplateScopeEnum._('unknownDefaultOpenApi');

TemplateScopeEnum _$templateScopeEnumValueOf(String name) {
  switch (name) {
    case 'personal':
      return _$templateScopeEnum_personal;
    case 'channel':
      return _$templateScopeEnum_channel;
    case 'unknownDefaultOpenApi':
      return _$templateScopeEnum_unknownDefaultOpenApi;
    default:
      return _$templateScopeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<TemplateScopeEnum> _$templateScopeEnumValues =
    BuiltSet<TemplateScopeEnum>(const <TemplateScopeEnum>[
      _$templateScopeEnum_personal,
      _$templateScopeEnum_channel,
      _$templateScopeEnum_unknownDefaultOpenApi,
    ]);

Serializer<TemplateScopeEnum> _$templateScopeEnumSerializer =
    _$TemplateScopeEnumSerializer();

class _$TemplateScopeEnumSerializer
    implements PrimitiveSerializer<TemplateScopeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'personal': 'personal',
    'channel': 'channel',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'personal': 'personal',
    'channel': 'channel',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TemplateScopeEnum];
  @override
  final String wireName = 'TemplateScopeEnum';

  @override
  Object serialize(
    Serializers serializers,
    TemplateScopeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TemplateScopeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TemplateScopeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Template extends Template {
  @override
  final String id;
  @override
  final TemplateScopeEnum scope;
  @override
  final String? channelId;
  @override
  final String name;
  @override
  final String contentMd;
  @override
  final int version;

  factory _$Template([void Function(TemplateBuilder)? updates]) =>
      (TemplateBuilder()..update(updates))._build();

  _$Template._({
    required this.id,
    required this.scope,
    this.channelId,
    required this.name,
    required this.contentMd,
    required this.version,
  }) : super._();
  @override
  Template rebuild(void Function(TemplateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TemplateBuilder toBuilder() => TemplateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Template &&
        id == other.id &&
        scope == other.scope &&
        channelId == other.channelId &&
        name == other.name &&
        contentMd == other.contentMd &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, scope.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Template')
          ..add('id', id)
          ..add('scope', scope)
          ..add('channelId', channelId)
          ..add('name', name)
          ..add('contentMd', contentMd)
          ..add('version', version))
        .toString();
  }
}

class TemplateBuilder implements Builder<Template, TemplateBuilder> {
  _$Template? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  TemplateScopeEnum? _scope;
  TemplateScopeEnum? get scope => _$this._scope;
  set scope(TemplateScopeEnum? scope) => _$this._scope = scope;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  TemplateBuilder() {
    Template._defaults(this);
  }

  TemplateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _scope = $v.scope;
      _channelId = $v.channelId;
      _name = $v.name;
      _contentMd = $v.contentMd;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Template other) {
    _$v = other as _$Template;
  }

  @override
  void update(void Function(TemplateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Template build() => _build();

  _$Template _build() {
    final _$result =
        _$v ??
        _$Template._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Template', 'id'),
          scope: BuiltValueNullFieldError.checkNotNull(
            scope,
            r'Template',
            'scope',
          ),
          channelId: channelId,
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'Template',
            'name',
          ),
          contentMd: BuiltValueNullFieldError.checkNotNull(
            contentMd,
            r'Template',
            'contentMd',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'Template',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
