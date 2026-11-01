// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'heartbeat_ack.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HeartbeatAck extends HeartbeatAck {
  @override
  final AgentRuntimeStatus presence;
  @override
  final int intervalMs;
  @override
  final BuiltList<ProjectTrashRequest>? projectTrash;

  factory _$HeartbeatAck([void Function(HeartbeatAckBuilder)? updates]) =>
      (HeartbeatAckBuilder()..update(updates))._build();

  _$HeartbeatAck._({
    required this.presence,
    required this.intervalMs,
    this.projectTrash,
  }) : super._();
  @override
  HeartbeatAck rebuild(void Function(HeartbeatAckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HeartbeatAckBuilder toBuilder() => HeartbeatAckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HeartbeatAck &&
        presence == other.presence &&
        intervalMs == other.intervalMs &&
        projectTrash == other.projectTrash;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, presence.hashCode);
    _$hash = $jc(_$hash, intervalMs.hashCode);
    _$hash = $jc(_$hash, projectTrash.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HeartbeatAck')
          ..add('presence', presence)
          ..add('intervalMs', intervalMs)
          ..add('projectTrash', projectTrash))
        .toString();
  }
}

class HeartbeatAckBuilder
    implements Builder<HeartbeatAck, HeartbeatAckBuilder> {
  _$HeartbeatAck? _$v;

  AgentRuntimeStatusBuilder? _presence;
  AgentRuntimeStatusBuilder get presence =>
      _$this._presence ??= AgentRuntimeStatusBuilder();
  set presence(AgentRuntimeStatusBuilder? presence) =>
      _$this._presence = presence;

  int? _intervalMs;
  int? get intervalMs => _$this._intervalMs;
  set intervalMs(int? intervalMs) => _$this._intervalMs = intervalMs;

  ListBuilder<ProjectTrashRequest>? _projectTrash;
  ListBuilder<ProjectTrashRequest> get projectTrash =>
      _$this._projectTrash ??= ListBuilder<ProjectTrashRequest>();
  set projectTrash(ListBuilder<ProjectTrashRequest>? projectTrash) =>
      _$this._projectTrash = projectTrash;

  HeartbeatAckBuilder() {
    HeartbeatAck._defaults(this);
  }

  HeartbeatAckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _presence = $v.presence.toBuilder();
      _intervalMs = $v.intervalMs;
      _projectTrash = $v.projectTrash?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HeartbeatAck other) {
    _$v = other as _$HeartbeatAck;
  }

  @override
  void update(void Function(HeartbeatAckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HeartbeatAck build() => _build();

  _$HeartbeatAck _build() {
    _$HeartbeatAck _$result;
    try {
      _$result =
          _$v ??
          _$HeartbeatAck._(
            presence: presence.build(),
            intervalMs: BuiltValueNullFieldError.checkNotNull(
              intervalMs,
              r'HeartbeatAck',
              'intervalMs',
            ),
            projectTrash: _projectTrash?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'presence';
        presence.build();

        _$failedField = 'projectTrash';
        _projectTrash?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HeartbeatAck',
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
