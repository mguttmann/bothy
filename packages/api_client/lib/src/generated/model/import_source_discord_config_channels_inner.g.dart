// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_discord_config_channels_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportSourceDiscordConfigChannelsInnerLeadEnum
_$importSourceDiscordConfigChannelsInnerLeadEnum_discord =
    const ImportSourceDiscordConfigChannelsInnerLeadEnum._('discord');
const ImportSourceDiscordConfigChannelsInnerLeadEnum
_$importSourceDiscordConfigChannelsInnerLeadEnum_bothy =
    const ImportSourceDiscordConfigChannelsInnerLeadEnum._('bothy');
const ImportSourceDiscordConfigChannelsInnerLeadEnum
_$importSourceDiscordConfigChannelsInnerLeadEnum_unknownDefaultOpenApi =
    const ImportSourceDiscordConfigChannelsInnerLeadEnum._(
      'unknownDefaultOpenApi',
    );

ImportSourceDiscordConfigChannelsInnerLeadEnum
_$importSourceDiscordConfigChannelsInnerLeadEnumValueOf(String name) {
  switch (name) {
    case 'discord':
      return _$importSourceDiscordConfigChannelsInnerLeadEnum_discord;
    case 'bothy':
      return _$importSourceDiscordConfigChannelsInnerLeadEnum_bothy;
    case 'unknownDefaultOpenApi':
      return _$importSourceDiscordConfigChannelsInnerLeadEnum_unknownDefaultOpenApi;
    default:
      return _$importSourceDiscordConfigChannelsInnerLeadEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportSourceDiscordConfigChannelsInnerLeadEnum>
_$importSourceDiscordConfigChannelsInnerLeadEnumValues =
    BuiltSet<ImportSourceDiscordConfigChannelsInnerLeadEnum>(
      const <ImportSourceDiscordConfigChannelsInnerLeadEnum>[
        _$importSourceDiscordConfigChannelsInnerLeadEnum_discord,
        _$importSourceDiscordConfigChannelsInnerLeadEnum_bothy,
        _$importSourceDiscordConfigChannelsInnerLeadEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ImportSourceDiscordConfigChannelsInnerLeadEnum>
_$importSourceDiscordConfigChannelsInnerLeadEnumSerializer =
    _$ImportSourceDiscordConfigChannelsInnerLeadEnumSerializer();

class _$ImportSourceDiscordConfigChannelsInnerLeadEnumSerializer
    implements
        PrimitiveSerializer<ImportSourceDiscordConfigChannelsInnerLeadEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'discord': 'discord',
    'bothy': 'bothy',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'discord': 'discord',
    'bothy': 'bothy',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ImportSourceDiscordConfigChannelsInnerLeadEnum,
  ];
  @override
  final String wireName = 'ImportSourceDiscordConfigChannelsInnerLeadEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceDiscordConfigChannelsInnerLeadEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportSourceDiscordConfigChannelsInnerLeadEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportSourceDiscordConfigChannelsInnerLeadEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportSourceDiscordConfigChannelsInner
    extends ImportSourceDiscordConfigChannelsInner {
  @override
  final String discordChannel;
  @override
  final JsonObject? target;
  @override
  final String? plannedOperator;
  @override
  final ImportSourceDiscordConfigChannelsInnerLeadEnum? lead;

  factory _$ImportSourceDiscordConfigChannelsInner([
    void Function(ImportSourceDiscordConfigChannelsInnerBuilder)? updates,
  ]) => (ImportSourceDiscordConfigChannelsInnerBuilder()..update(updates))
      ._build();

  _$ImportSourceDiscordConfigChannelsInner._({
    required this.discordChannel,
    this.target,
    this.plannedOperator,
    this.lead,
  }) : super._();
  @override
  ImportSourceDiscordConfigChannelsInner rebuild(
    void Function(ImportSourceDiscordConfigChannelsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceDiscordConfigChannelsInnerBuilder toBuilder() =>
      ImportSourceDiscordConfigChannelsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceDiscordConfigChannelsInner &&
        discordChannel == other.discordChannel &&
        target == other.target &&
        plannedOperator == other.plannedOperator &&
        lead == other.lead;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, discordChannel.hashCode);
    _$hash = $jc(_$hash, target.hashCode);
    _$hash = $jc(_$hash, plannedOperator.hashCode);
    _$hash = $jc(_$hash, lead.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
            r'ImportSourceDiscordConfigChannelsInner',
          )
          ..add('discordChannel', discordChannel)
          ..add('target', target)
          ..add('plannedOperator', plannedOperator)
          ..add('lead', lead))
        .toString();
  }
}

class ImportSourceDiscordConfigChannelsInnerBuilder
    implements
        Builder<
          ImportSourceDiscordConfigChannelsInner,
          ImportSourceDiscordConfigChannelsInnerBuilder
        > {
  _$ImportSourceDiscordConfigChannelsInner? _$v;

  String? _discordChannel;
  String? get discordChannel => _$this._discordChannel;
  set discordChannel(String? discordChannel) =>
      _$this._discordChannel = discordChannel;

  JsonObject? _target;
  JsonObject? get target => _$this._target;
  set target(JsonObject? target) => _$this._target = target;

  String? _plannedOperator;
  String? get plannedOperator => _$this._plannedOperator;
  set plannedOperator(String? plannedOperator) =>
      _$this._plannedOperator = plannedOperator;

  ImportSourceDiscordConfigChannelsInnerLeadEnum? _lead;
  ImportSourceDiscordConfigChannelsInnerLeadEnum? get lead => _$this._lead;
  set lead(ImportSourceDiscordConfigChannelsInnerLeadEnum? lead) =>
      _$this._lead = lead;

  ImportSourceDiscordConfigChannelsInnerBuilder() {
    ImportSourceDiscordConfigChannelsInner._defaults(this);
  }

  ImportSourceDiscordConfigChannelsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _discordChannel = $v.discordChannel;
      _target = $v.target;
      _plannedOperator = $v.plannedOperator;
      _lead = $v.lead;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceDiscordConfigChannelsInner other) {
    _$v = other as _$ImportSourceDiscordConfigChannelsInner;
  }

  @override
  void update(
    void Function(ImportSourceDiscordConfigChannelsInnerBuilder)? updates,
  ) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceDiscordConfigChannelsInner build() => _build();

  _$ImportSourceDiscordConfigChannelsInner _build() {
    final _$result =
        _$v ??
        _$ImportSourceDiscordConfigChannelsInner._(
          discordChannel: BuiltValueNullFieldError.checkNotNull(
            discordChannel,
            r'ImportSourceDiscordConfigChannelsInner',
            'discordChannel',
          ),
          target: target,
          plannedOperator: plannedOperator,
          lead: lead,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
