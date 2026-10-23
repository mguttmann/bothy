// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_channel_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportChannelDataKindEnum _$importChannelDataKindEnum_text =
    const ImportChannelDataKindEnum._('text');
const ImportChannelDataKindEnum _$importChannelDataKindEnum_forum =
    const ImportChannelDataKindEnum._('forum');
const ImportChannelDataKindEnum
_$importChannelDataKindEnum_unknownDefaultOpenApi =
    const ImportChannelDataKindEnum._('unknownDefaultOpenApi');

ImportChannelDataKindEnum _$importChannelDataKindEnumValueOf(String name) {
  switch (name) {
    case 'text':
      return _$importChannelDataKindEnum_text;
    case 'forum':
      return _$importChannelDataKindEnum_forum;
    case 'unknownDefaultOpenApi':
      return _$importChannelDataKindEnum_unknownDefaultOpenApi;
    default:
      return _$importChannelDataKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportChannelDataKindEnum> _$importChannelDataKindEnumValues =
    BuiltSet<ImportChannelDataKindEnum>(const <ImportChannelDataKindEnum>[
      _$importChannelDataKindEnum_text,
      _$importChannelDataKindEnum_forum,
      _$importChannelDataKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportChannelDataKindEnum> _$importChannelDataKindEnumSerializer =
    _$ImportChannelDataKindEnumSerializer();

class _$ImportChannelDataKindEnumSerializer
    implements PrimitiveSerializer<ImportChannelDataKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'text': 'text',
    'forum': 'forum',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'text': 'text',
    'forum': 'forum',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportChannelDataKindEnum];
  @override
  final String wireName = 'ImportChannelDataKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportChannelDataKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportChannelDataKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportChannelDataKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportChannelData extends ImportChannelData {
  @override
  final String? spaceOriginKey;
  @override
  final String? existingSpaceId;
  @override
  final String? name;
  @override
  final ImportChannelDataKindEnum kind;
  @override
  final String? topic;
  @override
  final String sortKey;
  @override
  final String? existingChannelId;

  factory _$ImportChannelData([
    void Function(ImportChannelDataBuilder)? updates,
  ]) => (ImportChannelDataBuilder()..update(updates))._build();

  _$ImportChannelData._({
    this.spaceOriginKey,
    this.existingSpaceId,
    this.name,
    required this.kind,
    this.topic,
    required this.sortKey,
    this.existingChannelId,
  }) : super._();
  @override
  ImportChannelData rebuild(void Function(ImportChannelDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportChannelDataBuilder toBuilder() =>
      ImportChannelDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportChannelData &&
        spaceOriginKey == other.spaceOriginKey &&
        existingSpaceId == other.existingSpaceId &&
        name == other.name &&
        kind == other.kind &&
        topic == other.topic &&
        sortKey == other.sortKey &&
        existingChannelId == other.existingChannelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, spaceOriginKey.hashCode);
    _$hash = $jc(_$hash, existingSpaceId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, topic.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, existingChannelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportChannelData')
          ..add('spaceOriginKey', spaceOriginKey)
          ..add('existingSpaceId', existingSpaceId)
          ..add('name', name)
          ..add('kind', kind)
          ..add('topic', topic)
          ..add('sortKey', sortKey)
          ..add('existingChannelId', existingChannelId))
        .toString();
  }
}

class ImportChannelDataBuilder
    implements Builder<ImportChannelData, ImportChannelDataBuilder> {
  _$ImportChannelData? _$v;

  String? _spaceOriginKey;
  String? get spaceOriginKey => _$this._spaceOriginKey;
  set spaceOriginKey(String? spaceOriginKey) =>
      _$this._spaceOriginKey = spaceOriginKey;

  String? _existingSpaceId;
  String? get existingSpaceId => _$this._existingSpaceId;
  set existingSpaceId(String? existingSpaceId) =>
      _$this._existingSpaceId = existingSpaceId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  ImportChannelDataKindEnum? _kind;
  ImportChannelDataKindEnum? get kind => _$this._kind;
  set kind(ImportChannelDataKindEnum? kind) => _$this._kind = kind;

  String? _topic;
  String? get topic => _$this._topic;
  set topic(String? topic) => _$this._topic = topic;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  String? _existingChannelId;
  String? get existingChannelId => _$this._existingChannelId;
  set existingChannelId(String? existingChannelId) =>
      _$this._existingChannelId = existingChannelId;

  ImportChannelDataBuilder() {
    ImportChannelData._defaults(this);
  }

  ImportChannelDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _spaceOriginKey = $v.spaceOriginKey;
      _existingSpaceId = $v.existingSpaceId;
      _name = $v.name;
      _kind = $v.kind;
      _topic = $v.topic;
      _sortKey = $v.sortKey;
      _existingChannelId = $v.existingChannelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportChannelData other) {
    _$v = other as _$ImportChannelData;
  }

  @override
  void update(void Function(ImportChannelDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportChannelData build() => _build();

  _$ImportChannelData _build() {
    final _$result =
        _$v ??
        _$ImportChannelData._(
          spaceOriginKey: spaceOriginKey,
          existingSpaceId: existingSpaceId,
          name: name,
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'ImportChannelData',
            'kind',
          ),
          topic: topic,
          sortKey: BuiltValueNullFieldError.checkNotNull(
            sortKey,
            r'ImportChannelData',
            'sortKey',
          ),
          existingChannelId: existingChannelId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
