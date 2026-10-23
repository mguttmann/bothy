// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_user_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportUserDataDiscordKindEnum _$importUserDataDiscordKindEnum_user =
    const ImportUserDataDiscordKindEnum._('user');
const ImportUserDataDiscordKindEnum _$importUserDataDiscordKindEnum_bot =
    const ImportUserDataDiscordKindEnum._('bot');
const ImportUserDataDiscordKindEnum _$importUserDataDiscordKindEnum_webhook =
    const ImportUserDataDiscordKindEnum._('webhook');
const ImportUserDataDiscordKindEnum
_$importUserDataDiscordKindEnum_unknownDefaultOpenApi =
    const ImportUserDataDiscordKindEnum._('unknownDefaultOpenApi');

ImportUserDataDiscordKindEnum _$importUserDataDiscordKindEnumValueOf(
  String name,
) {
  switch (name) {
    case 'user':
      return _$importUserDataDiscordKindEnum_user;
    case 'bot':
      return _$importUserDataDiscordKindEnum_bot;
    case 'webhook':
      return _$importUserDataDiscordKindEnum_webhook;
    case 'unknownDefaultOpenApi':
      return _$importUserDataDiscordKindEnum_unknownDefaultOpenApi;
    default:
      return _$importUserDataDiscordKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportUserDataDiscordKindEnum>
_$importUserDataDiscordKindEnumValues = BuiltSet<ImportUserDataDiscordKindEnum>(
  const <ImportUserDataDiscordKindEnum>[
    _$importUserDataDiscordKindEnum_user,
    _$importUserDataDiscordKindEnum_bot,
    _$importUserDataDiscordKindEnum_webhook,
    _$importUserDataDiscordKindEnum_unknownDefaultOpenApi,
  ],
);

Serializer<ImportUserDataDiscordKindEnum>
_$importUserDataDiscordKindEnumSerializer =
    _$ImportUserDataDiscordKindEnumSerializer();

class _$ImportUserDataDiscordKindEnumSerializer
    implements PrimitiveSerializer<ImportUserDataDiscordKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'user': 'user',
    'bot': 'bot',
    'webhook': 'webhook',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user': 'user',
    'bot': 'bot',
    'webhook': 'webhook',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportUserDataDiscordKindEnum];
  @override
  final String wireName = 'ImportUserDataDiscordKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportUserDataDiscordKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportUserDataDiscordKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportUserDataDiscordKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportUserData extends ImportUserData {
  @override
  final ImportUserDataDiscordKindEnum discordKind;
  @override
  final String username;
  @override
  final String? globalName;

  factory _$ImportUserData([void Function(ImportUserDataBuilder)? updates]) =>
      (ImportUserDataBuilder()..update(updates))._build();

  _$ImportUserData._({
    required this.discordKind,
    required this.username,
    this.globalName,
  }) : super._();
  @override
  ImportUserData rebuild(void Function(ImportUserDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportUserDataBuilder toBuilder() => ImportUserDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportUserData &&
        discordKind == other.discordKind &&
        username == other.username &&
        globalName == other.globalName;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, discordKind.hashCode);
    _$hash = $jc(_$hash, username.hashCode);
    _$hash = $jc(_$hash, globalName.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportUserData')
          ..add('discordKind', discordKind)
          ..add('username', username)
          ..add('globalName', globalName))
        .toString();
  }
}

class ImportUserDataBuilder
    implements Builder<ImportUserData, ImportUserDataBuilder> {
  _$ImportUserData? _$v;

  ImportUserDataDiscordKindEnum? _discordKind;
  ImportUserDataDiscordKindEnum? get discordKind => _$this._discordKind;
  set discordKind(ImportUserDataDiscordKindEnum? discordKind) =>
      _$this._discordKind = discordKind;

  String? _username;
  String? get username => _$this._username;
  set username(String? username) => _$this._username = username;

  String? _globalName;
  String? get globalName => _$this._globalName;
  set globalName(String? globalName) => _$this._globalName = globalName;

  ImportUserDataBuilder() {
    ImportUserData._defaults(this);
  }

  ImportUserDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _discordKind = $v.discordKind;
      _username = $v.username;
      _globalName = $v.globalName;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportUserData other) {
    _$v = other as _$ImportUserData;
  }

  @override
  void update(void Function(ImportUserDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportUserData build() => _build();

  _$ImportUserData _build() {
    final _$result =
        _$v ??
        _$ImportUserData._(
          discordKind: BuiltValueNullFieldError.checkNotNull(
            discordKind,
            r'ImportUserData',
            'discordKind',
          ),
          username: BuiltValueNullFieldError.checkNotNull(
            username,
            r'ImportUserData',
            'username',
          ),
          globalName: globalName,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
