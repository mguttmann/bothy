// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_discord_config_defaults.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
_$importSourceDiscordConfigDefaultsUnmappedHumanEnum_placeholder =
    const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum._('placeholder');
const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
_$importSourceDiscordConfigDefaultsUnmappedHumanEnum_skip =
    const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum._('skip');
const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
_$importSourceDiscordConfigDefaultsUnmappedHumanEnum_unknownDefaultOpenApi =
    const ImportSourceDiscordConfigDefaultsUnmappedHumanEnum._(
      'unknownDefaultOpenApi',
    );

ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
_$importSourceDiscordConfigDefaultsUnmappedHumanEnumValueOf(String name) {
  switch (name) {
    case 'placeholder':
      return _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_placeholder;
    case 'skip':
      return _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_skip;
    case 'unknownDefaultOpenApi':
      return _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_unknownDefaultOpenApi;
    default:
      return _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedHumanEnum>
_$importSourceDiscordConfigDefaultsUnmappedHumanEnumValues =
    BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedHumanEnum>(const <
      ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
    >[
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_placeholder,
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_skip,
      _$importSourceDiscordConfigDefaultsUnmappedHumanEnum_unknownDefaultOpenApi,
    ]);

const ImportSourceDiscordConfigDefaultsUnmappedBotEnum
_$importSourceDiscordConfigDefaultsUnmappedBotEnum_placeholder =
    const ImportSourceDiscordConfigDefaultsUnmappedBotEnum._('placeholder');
const ImportSourceDiscordConfigDefaultsUnmappedBotEnum
_$importSourceDiscordConfigDefaultsUnmappedBotEnum_skip =
    const ImportSourceDiscordConfigDefaultsUnmappedBotEnum._('skip');
const ImportSourceDiscordConfigDefaultsUnmappedBotEnum
_$importSourceDiscordConfigDefaultsUnmappedBotEnum_unknownDefaultOpenApi =
    const ImportSourceDiscordConfigDefaultsUnmappedBotEnum._(
      'unknownDefaultOpenApi',
    );

ImportSourceDiscordConfigDefaultsUnmappedBotEnum
_$importSourceDiscordConfigDefaultsUnmappedBotEnumValueOf(String name) {
  switch (name) {
    case 'placeholder':
      return _$importSourceDiscordConfigDefaultsUnmappedBotEnum_placeholder;
    case 'skip':
      return _$importSourceDiscordConfigDefaultsUnmappedBotEnum_skip;
    case 'unknownDefaultOpenApi':
      return _$importSourceDiscordConfigDefaultsUnmappedBotEnum_unknownDefaultOpenApi;
    default:
      return _$importSourceDiscordConfigDefaultsUnmappedBotEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedBotEnum>
_$importSourceDiscordConfigDefaultsUnmappedBotEnumValues =
    BuiltSet<ImportSourceDiscordConfigDefaultsUnmappedBotEnum>(const <
      ImportSourceDiscordConfigDefaultsUnmappedBotEnum
    >[
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_placeholder,
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_skip,
      _$importSourceDiscordConfigDefaultsUnmappedBotEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportSourceDiscordConfigDefaultsUnmappedHumanEnum>
_$importSourceDiscordConfigDefaultsUnmappedHumanEnumSerializer =
    _$ImportSourceDiscordConfigDefaultsUnmappedHumanEnumSerializer();
Serializer<ImportSourceDiscordConfigDefaultsUnmappedBotEnum>
_$importSourceDiscordConfigDefaultsUnmappedBotEnumSerializer =
    _$ImportSourceDiscordConfigDefaultsUnmappedBotEnumSerializer();

class _$ImportSourceDiscordConfigDefaultsUnmappedHumanEnumSerializer
    implements
        PrimitiveSerializer<
          ImportSourceDiscordConfigDefaultsUnmappedHumanEnum
        > {
  static const Map<String, Object> _toWire = const <String, Object>{
    'placeholder': 'placeholder',
    'skip': 'skip',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'placeholder': 'placeholder',
    'skip': 'skip',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ImportSourceDiscordConfigDefaultsUnmappedHumanEnum,
  ];
  @override
  final String wireName = 'ImportSourceDiscordConfigDefaultsUnmappedHumanEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigDefaultsUnmappedHumanEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportSourceDiscordConfigDefaultsUnmappedHumanEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportSourceDiscordConfigDefaultsUnmappedHumanEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportSourceDiscordConfigDefaultsUnmappedBotEnumSerializer
    implements
        PrimitiveSerializer<ImportSourceDiscordConfigDefaultsUnmappedBotEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'placeholder': 'placeholder',
    'skip': 'skip',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'placeholder': 'placeholder',
    'skip': 'skip',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ImportSourceDiscordConfigDefaultsUnmappedBotEnum,
  ];
  @override
  final String wireName = 'ImportSourceDiscordConfigDefaultsUnmappedBotEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigDefaultsUnmappedBotEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportSourceDiscordConfigDefaultsUnmappedBotEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportSourceDiscordConfigDefaultsUnmappedBotEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportSourceDiscordConfigDefaults
    extends ImportSourceDiscordConfigDefaults {
  @override
  final ImportSourceDiscordConfigDefaultsUnmappedHumanEnum unmappedHuman;
  @override
  final ImportSourceDiscordConfigDefaultsUnmappedBotEnum unmappedBot;
  @override
  final String uncategorizedSpace;

  factory _$ImportSourceDiscordConfigDefaults([
    void Function(ImportSourceDiscordConfigDefaultsBuilder)? updates,
  ]) => (ImportSourceDiscordConfigDefaultsBuilder()..update(updates))._build();

  _$ImportSourceDiscordConfigDefaults._({
    required this.unmappedHuman,
    required this.unmappedBot,
    required this.uncategorizedSpace,
  }) : super._();
  @override
  ImportSourceDiscordConfigDefaults rebuild(
    void Function(ImportSourceDiscordConfigDefaultsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceDiscordConfigDefaultsBuilder toBuilder() =>
      ImportSourceDiscordConfigDefaultsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceDiscordConfigDefaults &&
        unmappedHuman == other.unmappedHuman &&
        unmappedBot == other.unmappedBot &&
        uncategorizedSpace == other.uncategorizedSpace;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, unmappedHuman.hashCode);
    _$hash = $jc(_$hash, unmappedBot.hashCode);
    _$hash = $jc(_$hash, uncategorizedSpace.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSourceDiscordConfigDefaults')
          ..add('unmappedHuman', unmappedHuman)
          ..add('unmappedBot', unmappedBot)
          ..add('uncategorizedSpace', uncategorizedSpace))
        .toString();
  }
}

class ImportSourceDiscordConfigDefaultsBuilder
    implements
        Builder<
          ImportSourceDiscordConfigDefaults,
          ImportSourceDiscordConfigDefaultsBuilder
        > {
  _$ImportSourceDiscordConfigDefaults? _$v;

  ImportSourceDiscordConfigDefaultsUnmappedHumanEnum? _unmappedHuman;
  ImportSourceDiscordConfigDefaultsUnmappedHumanEnum? get unmappedHuman =>
      _$this._unmappedHuman;
  set unmappedHuman(
    ImportSourceDiscordConfigDefaultsUnmappedHumanEnum? unmappedHuman,
  ) => _$this._unmappedHuman = unmappedHuman;

  ImportSourceDiscordConfigDefaultsUnmappedBotEnum? _unmappedBot;
  ImportSourceDiscordConfigDefaultsUnmappedBotEnum? get unmappedBot =>
      _$this._unmappedBot;
  set unmappedBot(
    ImportSourceDiscordConfigDefaultsUnmappedBotEnum? unmappedBot,
  ) => _$this._unmappedBot = unmappedBot;

  String? _uncategorizedSpace;
  String? get uncategorizedSpace => _$this._uncategorizedSpace;
  set uncategorizedSpace(String? uncategorizedSpace) =>
      _$this._uncategorizedSpace = uncategorizedSpace;

  ImportSourceDiscordConfigDefaultsBuilder() {
    ImportSourceDiscordConfigDefaults._defaults(this);
  }

  ImportSourceDiscordConfigDefaultsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _unmappedHuman = $v.unmappedHuman;
      _unmappedBot = $v.unmappedBot;
      _uncategorizedSpace = $v.uncategorizedSpace;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceDiscordConfigDefaults other) {
    _$v = other as _$ImportSourceDiscordConfigDefaults;
  }

  @override
  void update(
    void Function(ImportSourceDiscordConfigDefaultsBuilder)? updates,
  ) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceDiscordConfigDefaults build() => _build();

  _$ImportSourceDiscordConfigDefaults _build() {
    final _$result =
        _$v ??
        _$ImportSourceDiscordConfigDefaults._(
          unmappedHuman: BuiltValueNullFieldError.checkNotNull(
            unmappedHuman,
            r'ImportSourceDiscordConfigDefaults',
            'unmappedHuman',
          ),
          unmappedBot: BuiltValueNullFieldError.checkNotNull(
            unmappedBot,
            r'ImportSourceDiscordConfigDefaults',
            'unmappedBot',
          ),
          uncategorizedSpace: BuiltValueNullFieldError.checkNotNull(
            uncategorizedSpace,
            r'ImportSourceDiscordConfigDefaults',
            'uncategorizedSpace',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
