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
  @override
  final String? reason;

  factory _$ProjectReadiness([
    void Function(ProjectReadinessBuilder)? updates,
  ]) => (ProjectReadinessBuilder()..update(updates))._build();

  _$ProjectReadiness._({
    required this.channelId,
    required this.ready,
    this.reason,
  }) : super._();
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
        ready == other.ready &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, ready.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectReadiness')
          ..add('channelId', channelId)
          ..add('ready', ready)
          ..add('reason', reason))
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

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ProjectReadinessBuilder() {
    ProjectReadiness._defaults(this);
  }

  ProjectReadinessBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _ready = $v.ready;
      _reason = $v.reason;
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
          reason: reason,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
