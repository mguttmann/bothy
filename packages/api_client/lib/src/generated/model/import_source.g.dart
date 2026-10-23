// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportSourceStateEnum _$importSourceStateEnum_draft =
    const ImportSourceStateEnum._('draft');
const ImportSourceStateEnum _$importSourceStateEnum_running =
    const ImportSourceStateEnum._('running');
const ImportSourceStateEnum _$importSourceStateEnum_paused =
    const ImportSourceStateEnum._('paused');
const ImportSourceStateEnum _$importSourceStateEnum_completed =
    const ImportSourceStateEnum._('completed');
const ImportSourceStateEnum _$importSourceStateEnum_failed =
    const ImportSourceStateEnum._('failed');
const ImportSourceStateEnum _$importSourceStateEnum_unknownDefaultOpenApi =
    const ImportSourceStateEnum._('unknownDefaultOpenApi');

ImportSourceStateEnum _$importSourceStateEnumValueOf(String name) {
  switch (name) {
    case 'draft':
      return _$importSourceStateEnum_draft;
    case 'running':
      return _$importSourceStateEnum_running;
    case 'paused':
      return _$importSourceStateEnum_paused;
    case 'completed':
      return _$importSourceStateEnum_completed;
    case 'failed':
      return _$importSourceStateEnum_failed;
    case 'unknownDefaultOpenApi':
      return _$importSourceStateEnum_unknownDefaultOpenApi;
    default:
      return _$importSourceStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportSourceStateEnum> _$importSourceStateEnumValues =
    BuiltSet<ImportSourceStateEnum>(const <ImportSourceStateEnum>[
      _$importSourceStateEnum_draft,
      _$importSourceStateEnum_running,
      _$importSourceStateEnum_paused,
      _$importSourceStateEnum_completed,
      _$importSourceStateEnum_failed,
      _$importSourceStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportSourceStateEnum> _$importSourceStateEnumSerializer =
    _$ImportSourceStateEnumSerializer();

class _$ImportSourceStateEnumSerializer
    implements PrimitiveSerializer<ImportSourceStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'draft': 'draft',
    'running': 'running',
    'paused': 'paused',
    'completed': 'completed',
    'failed': 'failed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'draft': 'draft',
    'running': 'running',
    'paused': 'paused',
    'completed': 'completed',
    'failed': 'failed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportSourceStateEnum];
  @override
  final String wireName = 'ImportSourceStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportSourceStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportSourceStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportSourceStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportSource extends ImportSource {
  @override
  final String id;
  @override
  final ImportKind kind;
  @override
  final String name;
  @override
  final BuiltMap<String, JsonObject?> config;
  @override
  final ImportSourceStateEnum state;
  @override
  final BuiltMap<String, JsonObject?> stats;
  @override
  final BuiltMap<String, JsonObject?>? cursor;
  @override
  final DateTime createdAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? finishedAt;

  factory _$ImportSource([void Function(ImportSourceBuilder)? updates]) =>
      (ImportSourceBuilder()..update(updates))._build();

  _$ImportSource._({
    required this.id,
    required this.kind,
    required this.name,
    required this.config,
    required this.state,
    required this.stats,
    this.cursor,
    required this.createdAt,
    this.startedAt,
    this.finishedAt,
  }) : super._();
  @override
  ImportSource rebuild(void Function(ImportSourceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportSourceBuilder toBuilder() => ImportSourceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSource &&
        id == other.id &&
        kind == other.kind &&
        name == other.name &&
        config == other.config &&
        state == other.state &&
        stats == other.stats &&
        cursor == other.cursor &&
        createdAt == other.createdAt &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, config.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stats.hashCode);
    _$hash = $jc(_$hash, cursor.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSource')
          ..add('id', id)
          ..add('kind', kind)
          ..add('name', name)
          ..add('config', config)
          ..add('state', state)
          ..add('stats', stats)
          ..add('cursor', cursor)
          ..add('createdAt', createdAt)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt))
        .toString();
  }
}

class ImportSourceBuilder
    implements Builder<ImportSource, ImportSourceBuilder> {
  _$ImportSource? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  ImportKind? _kind;
  ImportKind? get kind => _$this._kind;
  set kind(ImportKind? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  MapBuilder<String, JsonObject?>? _config;
  MapBuilder<String, JsonObject?> get config =>
      _$this._config ??= MapBuilder<String, JsonObject?>();
  set config(MapBuilder<String, JsonObject?>? config) =>
      _$this._config = config;

  ImportSourceStateEnum? _state;
  ImportSourceStateEnum? get state => _$this._state;
  set state(ImportSourceStateEnum? state) => _$this._state = state;

  MapBuilder<String, JsonObject?>? _stats;
  MapBuilder<String, JsonObject?> get stats =>
      _$this._stats ??= MapBuilder<String, JsonObject?>();
  set stats(MapBuilder<String, JsonObject?>? stats) => _$this._stats = stats;

  MapBuilder<String, JsonObject?>? _cursor;
  MapBuilder<String, JsonObject?> get cursor =>
      _$this._cursor ??= MapBuilder<String, JsonObject?>();
  set cursor(MapBuilder<String, JsonObject?>? cursor) =>
      _$this._cursor = cursor;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  ImportSourceBuilder() {
    ImportSource._defaults(this);
  }

  ImportSourceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _kind = $v.kind;
      _name = $v.name;
      _config = $v.config.toBuilder();
      _state = $v.state;
      _stats = $v.stats.toBuilder();
      _cursor = $v.cursor?.toBuilder();
      _createdAt = $v.createdAt;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSource other) {
    _$v = other as _$ImportSource;
  }

  @override
  void update(void Function(ImportSourceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportSource build() => _build();

  _$ImportSource _build() {
    _$ImportSource _$result;
    try {
      _$result =
          _$v ??
          _$ImportSource._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'ImportSource',
              'id',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'ImportSource',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ImportSource',
              'name',
            ),
            config: config.build(),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'ImportSource',
              'state',
            ),
            stats: stats.build(),
            cursor: _cursor?.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'ImportSource',
              'createdAt',
            ),
            startedAt: startedAt,
            finishedAt: finishedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'config';
        config.build();

        _$failedField = 'stats';
        stats.build();
        _$failedField = 'cursor';
        _cursor?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportSource',
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
