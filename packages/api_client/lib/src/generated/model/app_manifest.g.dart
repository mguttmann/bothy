// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppManifest extends AppManifest {
  @override
  final int schemaVersion;
  @override
  final String slug;
  @override
  final String name;
  @override
  final String version;
  @override
  final String? description;
  @override
  final String? publisher;
  @override
  final String? homepageUrl;
  @override
  final BuiltList<AppScope> scopes;
  @override
  final BuiltList<String>? eventSubscriptions;
  @override
  final BuiltList<CommandRegister>? commands;
  @override
  final BuiltList<WebhookSubscriptionCreate>? webhooks;

  factory _$AppManifest([void Function(AppManifestBuilder)? updates]) =>
      (AppManifestBuilder()..update(updates))._build();

  _$AppManifest._({
    required this.schemaVersion,
    required this.slug,
    required this.name,
    required this.version,
    this.description,
    this.publisher,
    this.homepageUrl,
    required this.scopes,
    this.eventSubscriptions,
    this.commands,
    this.webhooks,
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
        version == other.version &&
        description == other.description &&
        publisher == other.publisher &&
        homepageUrl == other.homepageUrl &&
        scopes == other.scopes &&
        eventSubscriptions == other.eventSubscriptions &&
        commands == other.commands &&
        webhooks == other.webhooks;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, schemaVersion.hashCode);
    _$hash = $jc(_$hash, slug.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, publisher.hashCode);
    _$hash = $jc(_$hash, homepageUrl.hashCode);
    _$hash = $jc(_$hash, scopes.hashCode);
    _$hash = $jc(_$hash, eventSubscriptions.hashCode);
    _$hash = $jc(_$hash, commands.hashCode);
    _$hash = $jc(_$hash, webhooks.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifest')
          ..add('schemaVersion', schemaVersion)
          ..add('slug', slug)
          ..add('name', name)
          ..add('version', version)
          ..add('description', description)
          ..add('publisher', publisher)
          ..add('homepageUrl', homepageUrl)
          ..add('scopes', scopes)
          ..add('eventSubscriptions', eventSubscriptions)
          ..add('commands', commands)
          ..add('webhooks', webhooks))
        .toString();
  }
}

class AppManifestBuilder implements Builder<AppManifest, AppManifestBuilder> {
  _$AppManifest? _$v;

  int? _schemaVersion;
  int? get schemaVersion => _$this._schemaVersion;
  set schemaVersion(int? schemaVersion) =>
      _$this._schemaVersion = schemaVersion;

  String? _slug;
  String? get slug => _$this._slug;
  set slug(String? slug) => _$this._slug = slug;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _version;
  String? get version => _$this._version;
  set version(String? version) => _$this._version = version;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _publisher;
  String? get publisher => _$this._publisher;
  set publisher(String? publisher) => _$this._publisher = publisher;

  String? _homepageUrl;
  String? get homepageUrl => _$this._homepageUrl;
  set homepageUrl(String? homepageUrl) => _$this._homepageUrl = homepageUrl;

  ListBuilder<AppScope>? _scopes;
  ListBuilder<AppScope> get scopes =>
      _$this._scopes ??= ListBuilder<AppScope>();
  set scopes(ListBuilder<AppScope>? scopes) => _$this._scopes = scopes;

  ListBuilder<String>? _eventSubscriptions;
  ListBuilder<String> get eventSubscriptions =>
      _$this._eventSubscriptions ??= ListBuilder<String>();
  set eventSubscriptions(ListBuilder<String>? eventSubscriptions) =>
      _$this._eventSubscriptions = eventSubscriptions;

  ListBuilder<CommandRegister>? _commands;
  ListBuilder<CommandRegister> get commands =>
      _$this._commands ??= ListBuilder<CommandRegister>();
  set commands(ListBuilder<CommandRegister>? commands) =>
      _$this._commands = commands;

  ListBuilder<WebhookSubscriptionCreate>? _webhooks;
  ListBuilder<WebhookSubscriptionCreate> get webhooks =>
      _$this._webhooks ??= ListBuilder<WebhookSubscriptionCreate>();
  set webhooks(ListBuilder<WebhookSubscriptionCreate>? webhooks) =>
      _$this._webhooks = webhooks;

  AppManifestBuilder() {
    AppManifest._defaults(this);
  }

  AppManifestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _schemaVersion = $v.schemaVersion;
      _slug = $v.slug;
      _name = $v.name;
      _version = $v.version;
      _description = $v.description;
      _publisher = $v.publisher;
      _homepageUrl = $v.homepageUrl;
      _scopes = $v.scopes.toBuilder();
      _eventSubscriptions = $v.eventSubscriptions?.toBuilder();
      _commands = $v.commands?.toBuilder();
      _webhooks = $v.webhooks?.toBuilder();
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
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'AppManifest',
              'version',
            ),
            description: description,
            publisher: publisher,
            homepageUrl: homepageUrl,
            scopes: scopes.build(),
            eventSubscriptions: _eventSubscriptions?.build(),
            commands: _commands?.build(),
            webhooks: _webhooks?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'scopes';
        scopes.build();
        _$failedField = 'eventSubscriptions';
        _eventSubscriptions?.build();
        _$failedField = 'commands';
        _commands?.build();
        _$failedField = 'webhooks';
        _webhooks?.build();
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
