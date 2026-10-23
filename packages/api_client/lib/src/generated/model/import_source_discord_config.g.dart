// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_discord_config.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportSourceDiscordConfigVersionEnum
_$importSourceDiscordConfigVersionEnum_number1 =
    const ImportSourceDiscordConfigVersionEnum._('number1');
const ImportSourceDiscordConfigVersionEnum
_$importSourceDiscordConfigVersionEnum_unknownDefaultOpenApi =
    const ImportSourceDiscordConfigVersionEnum._('unknownDefaultOpenApi');

ImportSourceDiscordConfigVersionEnum
_$importSourceDiscordConfigVersionEnumValueOf(String name) {
  switch (name) {
    case 'number1':
      return _$importSourceDiscordConfigVersionEnum_number1;
    case 'unknownDefaultOpenApi':
      return _$importSourceDiscordConfigVersionEnum_unknownDefaultOpenApi;
    default:
      return _$importSourceDiscordConfigVersionEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportSourceDiscordConfigVersionEnum>
_$importSourceDiscordConfigVersionEnumValues =
    BuiltSet<ImportSourceDiscordConfigVersionEnum>(
      const <ImportSourceDiscordConfigVersionEnum>[
        _$importSourceDiscordConfigVersionEnum_number1,
        _$importSourceDiscordConfigVersionEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ImportSourceDiscordConfigVersionEnum>
_$importSourceDiscordConfigVersionEnumSerializer =
    _$ImportSourceDiscordConfigVersionEnumSerializer();

class _$ImportSourceDiscordConfigVersionEnumSerializer
    implements PrimitiveSerializer<ImportSourceDiscordConfigVersionEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'number1': 1,
    'unknownDefaultOpenApi': 11184809,
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    1: 'number1',
    11184809: 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ImportSourceDiscordConfigVersionEnum,
  ];
  @override
  final String wireName = 'ImportSourceDiscordConfigVersionEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigVersionEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportSourceDiscordConfigVersionEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportSourceDiscordConfigVersionEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportSourceDiscordConfig extends ImportSourceDiscordConfig {
  @override
  final ImportSourceDiscordConfigVersionEnum version;
  @override
  final String guildId;
  @override
  final ImportSourceDiscordConfigDefaults defaults;
  @override
  final BuiltList<ImportSourceDiscordConfigSpacesInner>? spaces;
  @override
  final BuiltList<ImportSourceDiscordConfigChannelsInner>? channels;
  @override
  final BuiltList<ImportSourceDiscordConfigUsersInner>? users;

  factory _$ImportSourceDiscordConfig([
    void Function(ImportSourceDiscordConfigBuilder)? updates,
  ]) => (ImportSourceDiscordConfigBuilder()..update(updates))._build();

  _$ImportSourceDiscordConfig._({
    required this.version,
    required this.guildId,
    required this.defaults,
    this.spaces,
    this.channels,
    this.users,
  }) : super._();
  @override
  ImportSourceDiscordConfig rebuild(
    void Function(ImportSourceDiscordConfigBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceDiscordConfigBuilder toBuilder() =>
      ImportSourceDiscordConfigBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceDiscordConfig &&
        version == other.version &&
        guildId == other.guildId &&
        defaults == other.defaults &&
        spaces == other.spaces &&
        channels == other.channels &&
        users == other.users;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, guildId.hashCode);
    _$hash = $jc(_$hash, defaults.hashCode);
    _$hash = $jc(_$hash, spaces.hashCode);
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jc(_$hash, users.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSourceDiscordConfig')
          ..add('version', version)
          ..add('guildId', guildId)
          ..add('defaults', defaults)
          ..add('spaces', spaces)
          ..add('channels', channels)
          ..add('users', users))
        .toString();
  }
}

class ImportSourceDiscordConfigBuilder
    implements
        Builder<ImportSourceDiscordConfig, ImportSourceDiscordConfigBuilder> {
  _$ImportSourceDiscordConfig? _$v;

  ImportSourceDiscordConfigVersionEnum? _version;
  ImportSourceDiscordConfigVersionEnum? get version => _$this._version;
  set version(ImportSourceDiscordConfigVersionEnum? version) =>
      _$this._version = version;

  String? _guildId;
  String? get guildId => _$this._guildId;
  set guildId(String? guildId) => _$this._guildId = guildId;

  ImportSourceDiscordConfigDefaultsBuilder? _defaults;
  ImportSourceDiscordConfigDefaultsBuilder get defaults =>
      _$this._defaults ??= ImportSourceDiscordConfigDefaultsBuilder();
  set defaults(ImportSourceDiscordConfigDefaultsBuilder? defaults) =>
      _$this._defaults = defaults;

  ListBuilder<ImportSourceDiscordConfigSpacesInner>? _spaces;
  ListBuilder<ImportSourceDiscordConfigSpacesInner> get spaces =>
      _$this._spaces ??= ListBuilder<ImportSourceDiscordConfigSpacesInner>();
  set spaces(ListBuilder<ImportSourceDiscordConfigSpacesInner>? spaces) =>
      _$this._spaces = spaces;

  ListBuilder<ImportSourceDiscordConfigChannelsInner>? _channels;
  ListBuilder<ImportSourceDiscordConfigChannelsInner> get channels =>
      _$this._channels ??=
          ListBuilder<ImportSourceDiscordConfigChannelsInner>();
  set channels(ListBuilder<ImportSourceDiscordConfigChannelsInner>? channels) =>
      _$this._channels = channels;

  ListBuilder<ImportSourceDiscordConfigUsersInner>? _users;
  ListBuilder<ImportSourceDiscordConfigUsersInner> get users =>
      _$this._users ??= ListBuilder<ImportSourceDiscordConfigUsersInner>();
  set users(ListBuilder<ImportSourceDiscordConfigUsersInner>? users) =>
      _$this._users = users;

  ImportSourceDiscordConfigBuilder() {
    ImportSourceDiscordConfig._defaults(this);
  }

  ImportSourceDiscordConfigBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _version = $v.version;
      _guildId = $v.guildId;
      _defaults = $v.defaults.toBuilder();
      _spaces = $v.spaces?.toBuilder();
      _channels = $v.channels?.toBuilder();
      _users = $v.users?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceDiscordConfig other) {
    _$v = other as _$ImportSourceDiscordConfig;
  }

  @override
  void update(void Function(ImportSourceDiscordConfigBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceDiscordConfig build() => _build();

  _$ImportSourceDiscordConfig _build() {
    _$ImportSourceDiscordConfig _$result;
    try {
      _$result =
          _$v ??
          _$ImportSourceDiscordConfig._(
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'ImportSourceDiscordConfig',
              'version',
            ),
            guildId: BuiltValueNullFieldError.checkNotNull(
              guildId,
              r'ImportSourceDiscordConfig',
              'guildId',
            ),
            defaults: defaults.build(),
            spaces: _spaces?.build(),
            channels: _channels?.build(),
            users: _users?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'defaults';
        defaults.build();
        _$failedField = 'spaces';
        _spaces?.build();
        _$failedField = 'channels';
        _channels?.build();
        _$failedField = 'users';
        _users?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportSourceDiscordConfig',
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
