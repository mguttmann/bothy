// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessEntry extends AccessEntry {
  @override
  final PrincipalRef principal;
  @override
  final LevelOrNone level;
  @override
  final bool? inactive;
  @override
  final BuiltList<Capability> capabilities;
  @override
  final BuiltList<AccessSource> sources;

  factory _$AccessEntry([void Function(AccessEntryBuilder)? updates]) =>
      (AccessEntryBuilder()..update(updates))._build();

  _$AccessEntry._({
    required this.principal,
    required this.level,
    this.inactive,
    required this.capabilities,
    required this.sources,
  }) : super._();
  @override
  AccessEntry rebuild(void Function(AccessEntryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessEntryBuilder toBuilder() => AccessEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessEntry &&
        principal == other.principal &&
        level == other.level &&
        inactive == other.inactive &&
        capabilities == other.capabilities &&
        sources == other.sources;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, inactive.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jc(_$hash, sources.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessEntry')
          ..add('principal', principal)
          ..add('level', level)
          ..add('inactive', inactive)
          ..add('capabilities', capabilities)
          ..add('sources', sources))
        .toString();
  }
}

class AccessEntryBuilder implements Builder<AccessEntry, AccessEntryBuilder> {
  _$AccessEntry? _$v;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  LevelOrNone? _level;
  LevelOrNone? get level => _$this._level;
  set level(LevelOrNone? level) => _$this._level = level;

  bool? _inactive;
  bool? get inactive => _$this._inactive;
  set inactive(bool? inactive) => _$this._inactive = inactive;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  ListBuilder<AccessSource>? _sources;
  ListBuilder<AccessSource> get sources =>
      _$this._sources ??= ListBuilder<AccessSource>();
  set sources(ListBuilder<AccessSource>? sources) => _$this._sources = sources;

  AccessEntryBuilder() {
    AccessEntry._defaults(this);
  }

  AccessEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _principal = $v.principal.toBuilder();
      _level = $v.level;
      _inactive = $v.inactive;
      _capabilities = $v.capabilities.toBuilder();
      _sources = $v.sources.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessEntry other) {
    _$v = other as _$AccessEntry;
  }

  @override
  void update(void Function(AccessEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessEntry build() => _build();

  _$AccessEntry _build() {
    _$AccessEntry _$result;
    try {
      _$result =
          _$v ??
          _$AccessEntry._(
            principal: principal.build(),
            level: BuiltValueNullFieldError.checkNotNull(
              level,
              r'AccessEntry',
              'level',
            ),
            inactive: inactive,
            capabilities: capabilities.build(),
            sources: sources.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();

        _$failedField = 'capabilities';
        capabilities.build();
        _$failedField = 'sources';
        sources.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessEntry',
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
