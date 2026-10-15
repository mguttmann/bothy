// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_lead_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportLeadSetLeadEnum _$importLeadSetLeadEnum_discord =
    const ImportLeadSetLeadEnum._('discord');
const ImportLeadSetLeadEnum _$importLeadSetLeadEnum_bothy =
    const ImportLeadSetLeadEnum._('bothy');
const ImportLeadSetLeadEnum _$importLeadSetLeadEnum_unknownDefaultOpenApi =
    const ImportLeadSetLeadEnum._('unknownDefaultOpenApi');

ImportLeadSetLeadEnum _$importLeadSetLeadEnumValueOf(String name) {
  switch (name) {
    case 'discord':
      return _$importLeadSetLeadEnum_discord;
    case 'bothy':
      return _$importLeadSetLeadEnum_bothy;
    case 'unknownDefaultOpenApi':
      return _$importLeadSetLeadEnum_unknownDefaultOpenApi;
    default:
      return _$importLeadSetLeadEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportLeadSetLeadEnum> _$importLeadSetLeadEnumValues =
    BuiltSet<ImportLeadSetLeadEnum>(const <ImportLeadSetLeadEnum>[
      _$importLeadSetLeadEnum_discord,
      _$importLeadSetLeadEnum_bothy,
      _$importLeadSetLeadEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportLeadSetLeadEnum> _$importLeadSetLeadEnumSerializer =
    _$ImportLeadSetLeadEnumSerializer();

class _$ImportLeadSetLeadEnumSerializer
    implements PrimitiveSerializer<ImportLeadSetLeadEnum> {
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
  final Iterable<Type> types = const <Type>[ImportLeadSetLeadEnum];
  @override
  final String wireName = 'ImportLeadSetLeadEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportLeadSetLeadEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportLeadSetLeadEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportLeadSetLeadEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportLeadSet extends ImportLeadSet {
  @override
  final String sourceChannel;
  @override
  final ImportLeadSetLeadEnum lead;

  factory _$ImportLeadSet([void Function(ImportLeadSetBuilder)? updates]) =>
      (ImportLeadSetBuilder()..update(updates))._build();

  _$ImportLeadSet._({required this.sourceChannel, required this.lead})
    : super._();
  @override
  ImportLeadSet rebuild(void Function(ImportLeadSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportLeadSetBuilder toBuilder() => ImportLeadSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportLeadSet &&
        sourceChannel == other.sourceChannel &&
        lead == other.lead;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sourceChannel.hashCode);
    _$hash = $jc(_$hash, lead.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportLeadSet')
          ..add('sourceChannel', sourceChannel)
          ..add('lead', lead))
        .toString();
  }
}

class ImportLeadSetBuilder
    implements Builder<ImportLeadSet, ImportLeadSetBuilder> {
  _$ImportLeadSet? _$v;

  String? _sourceChannel;
  String? get sourceChannel => _$this._sourceChannel;
  set sourceChannel(String? sourceChannel) =>
      _$this._sourceChannel = sourceChannel;

  ImportLeadSetLeadEnum? _lead;
  ImportLeadSetLeadEnum? get lead => _$this._lead;
  set lead(ImportLeadSetLeadEnum? lead) => _$this._lead = lead;

  ImportLeadSetBuilder() {
    ImportLeadSet._defaults(this);
  }

  ImportLeadSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sourceChannel = $v.sourceChannel;
      _lead = $v.lead;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportLeadSet other) {
    _$v = other as _$ImportLeadSet;
  }

  @override
  void update(void Function(ImportLeadSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportLeadSet build() => _build();

  _$ImportLeadSet _build() {
    final _$result =
        _$v ??
        _$ImportLeadSet._(
          sourceChannel: BuiltValueNullFieldError.checkNotNull(
            sourceChannel,
            r'ImportLeadSet',
            'sourceChannel',
          ),
          lead: BuiltValueNullFieldError.checkNotNull(
            lead,
            r'ImportLeadSet',
            'lead',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
