// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_cursor_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportCursorSet extends ImportCursorSet {
  @override
  final BuiltMap<String, JsonObject?> cursor;
  @override
  final BuiltMap<String, JsonObject?>? stats;

  factory _$ImportCursorSet([void Function(ImportCursorSetBuilder)? updates]) =>
      (ImportCursorSetBuilder()..update(updates))._build();

  _$ImportCursorSet._({required this.cursor, this.stats}) : super._();
  @override
  ImportCursorSet rebuild(void Function(ImportCursorSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportCursorSetBuilder toBuilder() => ImportCursorSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportCursorSet &&
        cursor == other.cursor &&
        stats == other.stats;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, cursor.hashCode);
    _$hash = $jc(_$hash, stats.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportCursorSet')
          ..add('cursor', cursor)
          ..add('stats', stats))
        .toString();
  }
}

class ImportCursorSetBuilder
    implements Builder<ImportCursorSet, ImportCursorSetBuilder> {
  _$ImportCursorSet? _$v;

  MapBuilder<String, JsonObject?>? _cursor;
  MapBuilder<String, JsonObject?> get cursor =>
      _$this._cursor ??= MapBuilder<String, JsonObject?>();
  set cursor(MapBuilder<String, JsonObject?>? cursor) =>
      _$this._cursor = cursor;

  MapBuilder<String, JsonObject?>? _stats;
  MapBuilder<String, JsonObject?> get stats =>
      _$this._stats ??= MapBuilder<String, JsonObject?>();
  set stats(MapBuilder<String, JsonObject?>? stats) => _$this._stats = stats;

  ImportCursorSetBuilder() {
    ImportCursorSet._defaults(this);
  }

  ImportCursorSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _cursor = $v.cursor.toBuilder();
      _stats = $v.stats?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportCursorSet other) {
    _$v = other as _$ImportCursorSet;
  }

  @override
  void update(void Function(ImportCursorSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportCursorSet build() => _build();

  _$ImportCursorSet _build() {
    _$ImportCursorSet _$result;
    try {
      _$result =
          _$v ??
          _$ImportCursorSet._(cursor: cursor.build(), stats: _stats?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cursor';
        cursor.build();
        _$failedField = 'stats';
        _stats?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportCursorSet',
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
