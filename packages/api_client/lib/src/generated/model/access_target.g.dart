// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_target.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessTarget extends AccessTarget {
  @override
  final AccessTargetKind kind;
  @override
  final String id;
  @override
  final Level? level;
  @override
  final Level? previousLevel;
  @override
  final BuiltList<Capability>? capabilities;

  factory _$AccessTarget([void Function(AccessTargetBuilder)? updates]) =>
      (AccessTargetBuilder()..update(updates))._build();

  _$AccessTarget._({
    required this.kind,
    required this.id,
    this.level,
    this.previousLevel,
    this.capabilities,
  }) : super._();
  @override
  AccessTarget rebuild(void Function(AccessTargetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessTargetBuilder toBuilder() => AccessTargetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessTarget &&
        kind == other.kind &&
        id == other.id &&
        level == other.level &&
        previousLevel == other.previousLevel &&
        capabilities == other.capabilities;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, previousLevel.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessTarget')
          ..add('kind', kind)
          ..add('id', id)
          ..add('level', level)
          ..add('previousLevel', previousLevel)
          ..add('capabilities', capabilities))
        .toString();
  }
}

class AccessTargetBuilder
    implements Builder<AccessTarget, AccessTargetBuilder> {
  _$AccessTarget? _$v;

  AccessTargetKind? _kind;
  AccessTargetKind? get kind => _$this._kind;
  set kind(AccessTargetKind? kind) => _$this._kind = kind;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  Level? _level;
  Level? get level => _$this._level;
  set level(Level? level) => _$this._level = level;

  Level? _previousLevel;
  Level? get previousLevel => _$this._previousLevel;
  set previousLevel(Level? previousLevel) =>
      _$this._previousLevel = previousLevel;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  AccessTargetBuilder() {
    AccessTarget._defaults(this);
  }

  AccessTargetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _id = $v.id;
      _level = $v.level;
      _previousLevel = $v.previousLevel;
      _capabilities = $v.capabilities?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessTarget other) {
    _$v = other as _$AccessTarget;
  }

  @override
  void update(void Function(AccessTargetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessTarget build() => _build();

  _$AccessTarget _build() {
    _$AccessTarget _$result;
    try {
      _$result =
          _$v ??
          _$AccessTarget._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'AccessTarget',
              'kind',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'AccessTarget',
              'id',
            ),
            level: level,
            previousLevel: previousLevel,
            capabilities: _capabilities?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'capabilities';
        _capabilities?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessTarget',
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
