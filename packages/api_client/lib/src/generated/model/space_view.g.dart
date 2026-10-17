// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space_view.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SpaceView extends SpaceView {
  @override
  final Space space;
  @override
  final Level level;
  @override
  final BuiltList<Capability> capabilities;

  factory _$SpaceView([void Function(SpaceViewBuilder)? updates]) =>
      (SpaceViewBuilder()..update(updates))._build();

  _$SpaceView._({
    required this.space,
    required this.level,
    required this.capabilities,
  }) : super._();
  @override
  SpaceView rebuild(void Function(SpaceViewBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpaceViewBuilder toBuilder() => SpaceViewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SpaceView &&
        space == other.space &&
        level == other.level &&
        capabilities == other.capabilities;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, space.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SpaceView')
          ..add('space', space)
          ..add('level', level)
          ..add('capabilities', capabilities))
        .toString();
  }
}

class SpaceViewBuilder implements Builder<SpaceView, SpaceViewBuilder> {
  _$SpaceView? _$v;

  SpaceBuilder? _space;
  SpaceBuilder get space => _$this._space ??= SpaceBuilder();
  set space(SpaceBuilder? space) => _$this._space = space;

  Level? _level;
  Level? get level => _$this._level;
  set level(Level? level) => _$this._level = level;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  SpaceViewBuilder() {
    SpaceView._defaults(this);
  }

  SpaceViewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _space = $v.space.toBuilder();
      _level = $v.level;
      _capabilities = $v.capabilities.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SpaceView other) {
    _$v = other as _$SpaceView;
  }

  @override
  void update(void Function(SpaceViewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SpaceView build() => _build();

  _$SpaceView _build() {
    _$SpaceView _$result;
    try {
      _$result =
          _$v ??
          _$SpaceView._(
            space: space.build(),
            level: BuiltValueNullFieldError.checkNotNull(
              level,
              r'SpaceView',
              'level',
            ),
            capabilities: capabilities.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'space';
        space.build();

        _$failedField = 'capabilities';
        capabilities.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SpaceView',
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
