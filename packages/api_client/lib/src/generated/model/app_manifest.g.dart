// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AppManifestSchemaVersionEnum _$appManifestSchemaVersionEnum_number1 =
    const AppManifestSchemaVersionEnum._('number1');
const AppManifestSchemaVersionEnum
_$appManifestSchemaVersionEnum_unknownDefaultOpenApi =
    const AppManifestSchemaVersionEnum._('unknownDefaultOpenApi');

AppManifestSchemaVersionEnum _$appManifestSchemaVersionEnumValueOf(
  String name,
) {
  switch (name) {
    case 'number1':
      return _$appManifestSchemaVersionEnum_number1;
    case 'unknownDefaultOpenApi':
      return _$appManifestSchemaVersionEnum_unknownDefaultOpenApi;
    default:
      return _$appManifestSchemaVersionEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AppManifestSchemaVersionEnum>
_$appManifestSchemaVersionEnumValues = BuiltSet<AppManifestSchemaVersionEnum>(
  const <AppManifestSchemaVersionEnum>[
    _$appManifestSchemaVersionEnum_number1,
    _$appManifestSchemaVersionEnum_unknownDefaultOpenApi,
  ],
);

const AppManifestComponentsEnum _$appManifestComponentsEnum_button =
    const AppManifestComponentsEnum._('button');
const AppManifestComponentsEnum _$appManifestComponentsEnum_select =
    const AppManifestComponentsEnum._('select');
const AppManifestComponentsEnum _$appManifestComponentsEnum_form =
    const AppManifestComponentsEnum._('form');
const AppManifestComponentsEnum
_$appManifestComponentsEnum_unknownDefaultOpenApi =
    const AppManifestComponentsEnum._('unknownDefaultOpenApi');

AppManifestComponentsEnum _$appManifestComponentsEnumValueOf(String name) {
  switch (name) {
    case 'button':
      return _$appManifestComponentsEnum_button;
    case 'select':
      return _$appManifestComponentsEnum_select;
    case 'form':
      return _$appManifestComponentsEnum_form;
    case 'unknownDefaultOpenApi':
      return _$appManifestComponentsEnum_unknownDefaultOpenApi;
    default:
      return _$appManifestComponentsEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AppManifestComponentsEnum> _$appManifestComponentsEnumValues =
    BuiltSet<AppManifestComponentsEnum>(const <AppManifestComponentsEnum>[
      _$appManifestComponentsEnum_button,
      _$appManifestComponentsEnum_select,
      _$appManifestComponentsEnum_form,
      _$appManifestComponentsEnum_unknownDefaultOpenApi,
    ]);

Serializer<AppManifestSchemaVersionEnum>
_$appManifestSchemaVersionEnumSerializer =
    _$AppManifestSchemaVersionEnumSerializer();
Serializer<AppManifestComponentsEnum> _$appManifestComponentsEnumSerializer =
    _$AppManifestComponentsEnumSerializer();

class _$AppManifestSchemaVersionEnumSerializer
    implements PrimitiveSerializer<AppManifestSchemaVersionEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'number1': 1,
    'unknownDefaultOpenApi': 11184809,
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    1: 'number1',
    11184809: 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AppManifestSchemaVersionEnum];
  @override
  final String wireName = 'AppManifestSchemaVersionEnum';

  @override
  Object serialize(
    Serializers serializers,
    AppManifestSchemaVersionEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AppManifestSchemaVersionEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AppManifestSchemaVersionEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AppManifestComponentsEnumSerializer
    implements PrimitiveSerializer<AppManifestComponentsEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'button': 'button',
    'select': 'select',
    'form': 'form',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'button': 'button',
    'select': 'select',
    'form': 'form',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AppManifestComponentsEnum];
  @override
  final String wireName = 'AppManifestComponentsEnum';

  @override
  Object serialize(
    Serializers serializers,
    AppManifestComponentsEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AppManifestComponentsEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AppManifestComponentsEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AppManifest extends AppManifest {
  @override
  final AppManifestSchemaVersionEnum schemaVersion;
  @override
  final String slug;
  @override
  final String name;
  @override
  final String description;
  @override
  final String publisher;
  @override
  final String version;
  @override
  final String? homepageUrl;
  @override
  final String minApiVersion;
  @override
  final AppManifestScopes scopes;
  @override
  final BuiltList<String>? events;
  @override
  final AppManifestDelivery? delivery;
  @override
  final BuiltList<CommandRegister>? commands;
  @override
  final BuiltList<AppManifestComponentsEnum>? components;
  @override
  final BuiltList<AppManifestRequestedGrantsInner>? requestedGrants;
  @override
  final BuiltList<AppManifestIncomingWebhooksInner>? incomingWebhooks;
  @override
  final String? dataEgress;

  factory _$AppManifest([void Function(AppManifestBuilder)? updates]) =>
      (AppManifestBuilder()..update(updates))._build();

  _$AppManifest._({
    required this.schemaVersion,
    required this.slug,
    required this.name,
    required this.description,
    required this.publisher,
    required this.version,
    this.homepageUrl,
    required this.minApiVersion,
    required this.scopes,
    this.events,
    this.delivery,
    this.commands,
    this.components,
    this.requestedGrants,
    this.incomingWebhooks,
    this.dataEgress,
  }) : super._();
  @override
  AppManifest rebuild(void Function(AppManifestBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppManifestBuilder toBuilder() => AppManifestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppManifest &&
        schemaVersion == other.schemaVersion &&
        slug == other.slug &&
        name == other.name &&
        description == other.description &&
        publisher == other.publisher &&
        version == other.version &&
        homepageUrl == other.homepageUrl &&
        minApiVersion == other.minApiVersion &&
        scopes == other.scopes &&
        events == other.events &&
        delivery == other.delivery &&
        commands == other.commands &&
        components == other.components &&
        requestedGrants == other.requestedGrants &&
        incomingWebhooks == other.incomingWebhooks &&
        dataEgress == other.dataEgress;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, schemaVersion.hashCode);
    _$hash = $jc(_$hash, slug.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, publisher.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, homepageUrl.hashCode);
    _$hash = $jc(_$hash, minApiVersion.hashCode);
    _$hash = $jc(_$hash, scopes.hashCode);
    _$hash = $jc(_$hash, events.hashCode);
    _$hash = $jc(_$hash, delivery.hashCode);
    _$hash = $jc(_$hash, commands.hashCode);
    _$hash = $jc(_$hash, components.hashCode);
    _$hash = $jc(_$hash, requestedGrants.hashCode);
    _$hash = $jc(_$hash, incomingWebhooks.hashCode);
    _$hash = $jc(_$hash, dataEgress.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifest')
          ..add('schemaVersion', schemaVersion)
          ..add('slug', slug)
          ..add('name', name)
          ..add('description', description)
          ..add('publisher', publisher)
          ..add('version', version)
          ..add('homepageUrl', homepageUrl)
          ..add('minApiVersion', minApiVersion)
          ..add('scopes', scopes)
          ..add('events', events)
          ..add('delivery', delivery)
          ..add('commands', commands)
          ..add('components', components)
          ..add('requestedGrants', requestedGrants)
          ..add('incomingWebhooks', incomingWebhooks)
          ..add('dataEgress', dataEgress))
        .toString();
  }
}

class AppManifestBuilder implements Builder<AppManifest, AppManifestBuilder> {
  _$AppManifest? _$v;

  AppManifestSchemaVersionEnum? _schemaVersion;
  AppManifestSchemaVersionEnum? get schemaVersion => _$this._schemaVersion;
  set schemaVersion(AppManifestSchemaVersionEnum? schemaVersion) =>
      _$this._schemaVersion = schemaVersion;

  String? _slug;
  String? get slug => _$this._slug;
  set slug(String? slug) => _$this._slug = slug;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _publisher;
  String? get publisher => _$this._publisher;
  set publisher(String? publisher) => _$this._publisher = publisher;

  String? _version;
  String? get version => _$this._version;
  set version(String? version) => _$this._version = version;

  String? _homepageUrl;
  String? get homepageUrl => _$this._homepageUrl;
  set homepageUrl(String? homepageUrl) => _$this._homepageUrl = homepageUrl;

  String? _minApiVersion;
  String? get minApiVersion => _$this._minApiVersion;
  set minApiVersion(String? minApiVersion) =>
      _$this._minApiVersion = minApiVersion;

  AppManifestScopesBuilder? _scopes;
  AppManifestScopesBuilder get scopes =>
      _$this._scopes ??= AppManifestScopesBuilder();
  set scopes(AppManifestScopesBuilder? scopes) => _$this._scopes = scopes;

  ListBuilder<String>? _events;
  ListBuilder<String> get events => _$this._events ??= ListBuilder<String>();
  set events(ListBuilder<String>? events) => _$this._events = events;

  AppManifestDeliveryBuilder? _delivery;
  AppManifestDeliveryBuilder get delivery =>
      _$this._delivery ??= AppManifestDeliveryBuilder();
  set delivery(AppManifestDeliveryBuilder? delivery) =>
      _$this._delivery = delivery;

  ListBuilder<CommandRegister>? _commands;
  ListBuilder<CommandRegister> get commands =>
      _$this._commands ??= ListBuilder<CommandRegister>();
  set commands(ListBuilder<CommandRegister>? commands) =>
      _$this._commands = commands;

  ListBuilder<AppManifestComponentsEnum>? _components;
  ListBuilder<AppManifestComponentsEnum> get components =>
      _$this._components ??= ListBuilder<AppManifestComponentsEnum>();
  set components(ListBuilder<AppManifestComponentsEnum>? components) =>
      _$this._components = components;

  ListBuilder<AppManifestRequestedGrantsInner>? _requestedGrants;
  ListBuilder<AppManifestRequestedGrantsInner> get requestedGrants =>
      _$this._requestedGrants ??=
          ListBuilder<AppManifestRequestedGrantsInner>();
  set requestedGrants(
    ListBuilder<AppManifestRequestedGrantsInner>? requestedGrants,
  ) => _$this._requestedGrants = requestedGrants;

  ListBuilder<AppManifestIncomingWebhooksInner>? _incomingWebhooks;
  ListBuilder<AppManifestIncomingWebhooksInner> get incomingWebhooks =>
      _$this._incomingWebhooks ??=
          ListBuilder<AppManifestIncomingWebhooksInner>();
  set incomingWebhooks(
    ListBuilder<AppManifestIncomingWebhooksInner>? incomingWebhooks,
  ) => _$this._incomingWebhooks = incomingWebhooks;

  String? _dataEgress;
  String? get dataEgress => _$this._dataEgress;
  set dataEgress(String? dataEgress) => _$this._dataEgress = dataEgress;

  AppManifestBuilder() {
    AppManifest._defaults(this);
  }

  AppManifestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _schemaVersion = $v.schemaVersion;
      _slug = $v.slug;
      _name = $v.name;
      _description = $v.description;
      _publisher = $v.publisher;
      _version = $v.version;
      _homepageUrl = $v.homepageUrl;
      _minApiVersion = $v.minApiVersion;
      _scopes = $v.scopes.toBuilder();
      _events = $v.events?.toBuilder();
      _delivery = $v.delivery?.toBuilder();
      _commands = $v.commands?.toBuilder();
      _components = $v.components?.toBuilder();
      _requestedGrants = $v.requestedGrants?.toBuilder();
      _incomingWebhooks = $v.incomingWebhooks?.toBuilder();
      _dataEgress = $v.dataEgress;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppManifest other) {
    _$v = other as _$AppManifest;
  }

  @override
  void update(void Function(AppManifestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppManifest build() => _build();

  _$AppManifest _build() {
    _$AppManifest _$result;
    try {
      _$result =
          _$v ??
          _$AppManifest._(
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'AppManifest',
              'schemaVersion',
            ),
            slug: BuiltValueNullFieldError.checkNotNull(
              slug,
              r'AppManifest',
              'slug',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AppManifest',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'AppManifest',
              'description',
            ),
            publisher: BuiltValueNullFieldError.checkNotNull(
              publisher,
              r'AppManifest',
              'publisher',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'AppManifest',
              'version',
            ),
            homepageUrl: homepageUrl,
            minApiVersion: BuiltValueNullFieldError.checkNotNull(
              minApiVersion,
              r'AppManifest',
              'minApiVersion',
            ),
            scopes: scopes.build(),
            events: _events?.build(),
            delivery: _delivery?.build(),
            commands: _commands?.build(),
            components: _components?.build(),
            requestedGrants: _requestedGrants?.build(),
            incomingWebhooks: _incomingWebhooks?.build(),
            dataEgress: dataEgress,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'scopes';
        scopes.build();
        _$failedField = 'events';
        _events?.build();
        _$failedField = 'delivery';
        _delivery?.build();
        _$failedField = 'commands';
        _commands?.build();
        _$failedField = 'components';
        _components?.build();
        _$failedField = 'requestedGrants';
        _requestedGrants?.build();
        _$failedField = 'incomingWebhooks';
        _incomingWebhooks?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppManifest',
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
