// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_readiness.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectReadiness extends ProjectReadiness {
  @override
  final String channelId;
  @override
  final bool ready;

  factory _$ProjectReadiness([
    void Function(ProjectReadinessBuilder)? updates,
  ]) => (ProjectReadinessBuilder()..update(updates))._build();

  _$ProjectReadiness._({required this.channelId, required this.ready})
    : super._();
  @override
  ProjectReadiness rebuild(void Function(ProjectReadinessBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectReadinessBuilder toBuilder() =>
      ProjectReadinessBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectReadiness &&
        channelId == other.channelId &&
        ready == other.ready;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, ready.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectReadiness')
          ..add('channelId', channelId)
          ..add('ready', ready))
        .toString();
  }
}

class ProjectReadinessBuilder
    implements Builder<ProjectReadiness, ProjectReadinessBuilder> {
  _$ProjectReadiness? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  bool? _ready;
  bool? get ready => _$this._ready;
  set ready(bool? ready) => _$this._ready = ready;

  ProjectReadinessBuilder() {
    ProjectReadiness._defaults(this);
  }

  ProjectReadinessBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _ready = $v.ready;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectReadiness other) {
    _$v = other as _$ProjectReadiness;
  }

  @override
  void update(void Function(ProjectReadinessBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectReadiness build() => _build();

  _$ProjectReadiness _build() {
    final _$result =
        _$v ??
        _$ProjectReadiness._(
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ProjectReadiness',
            'channelId',
          ),
          ready: BuiltValueNullFieldError.checkNotNull(
            ready,
            r'ProjectReadiness',
            'ready',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
