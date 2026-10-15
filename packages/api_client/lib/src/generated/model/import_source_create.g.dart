// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportSourceCreate extends ImportSourceCreate {
  @override
  final ImportKind kind;
  @override
  final String name;
  @override
  final BuiltMap<String, JsonObject?> config;

  factory _$ImportSourceCreate([
    void Function(ImportSourceCreateBuilder)? updates,
  ]) => (ImportSourceCreateBuilder()..update(updates))._build();

  _$ImportSourceCreate._({
    required this.kind,
    required this.name,
    required this.config,
  }) : super._();
  @override
  ImportSourceCreate rebuild(
    void Function(ImportSourceCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceCreateBuilder toBuilder() =>
      ImportSourceCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceCreate &&
        kind == other.kind &&
        name == other.name &&
        config == other.config;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, config.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSourceCreate')
          ..add('kind', kind)
          ..add('name', name)
          ..add('config', config))
        .toString();
  }
}

class ImportSourceCreateBuilder
    implements Builder<ImportSourceCreate, ImportSourceCreateBuilder> {
  _$ImportSourceCreate? _$v;

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

  ImportSourceCreateBuilder() {
    ImportSourceCreate._defaults(this);
  }

  ImportSourceCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _name = $v.name;
      _config = $v.config.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceCreate other) {
    _$v = other as _$ImportSourceCreate;
  }

  @override
  void update(void Function(ImportSourceCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceCreate build() => _build();

  _$ImportSourceCreate _build() {
    _$ImportSourceCreate _$result;
    try {
      _$result =
          _$v ??
          _$ImportSourceCreate._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'ImportSourceCreate',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ImportSourceCreate',
              'name',
            ),
            config: config.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'config';
        config.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportSourceCreate',
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
