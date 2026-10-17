// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostUpdate extends HostUpdate {
  @override
  final String? name;
  @override
  final String? assignedUserId;
  @override
  final bool? clearAssignedUser;
  @override
  final BuiltList<HostSessionUserAssign>? addSessionUsers;
  @override
  final BuiltList<String>? revokeSessionUsers;
  @override
  final String reason;

  factory _$HostUpdate([void Function(HostUpdateBuilder)? updates]) =>
      (HostUpdateBuilder()..update(updates))._build();

  _$HostUpdate._({
    this.name,
    this.assignedUserId,
    this.clearAssignedUser,
    this.addSessionUsers,
    this.revokeSessionUsers,
    required this.reason,
  }) : super._();
  @override
  HostUpdate rebuild(void Function(HostUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostUpdateBuilder toBuilder() => HostUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostUpdate &&
        name == other.name &&
        assignedUserId == other.assignedUserId &&
        clearAssignedUser == other.clearAssignedUser &&
        addSessionUsers == other.addSessionUsers &&
        revokeSessionUsers == other.revokeSessionUsers &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, assignedUserId.hashCode);
    _$hash = $jc(_$hash, clearAssignedUser.hashCode);
    _$hash = $jc(_$hash, addSessionUsers.hashCode);
    _$hash = $jc(_$hash, revokeSessionUsers.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostUpdate')
          ..add('name', name)
          ..add('assignedUserId', assignedUserId)
          ..add('clearAssignedUser', clearAssignedUser)
          ..add('addSessionUsers', addSessionUsers)
          ..add('revokeSessionUsers', revokeSessionUsers)
          ..add('reason', reason))
        .toString();
  }
}

class HostUpdateBuilder implements Builder<HostUpdate, HostUpdateBuilder> {
  _$HostUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _assignedUserId;
  String? get assignedUserId => _$this._assignedUserId;
  set assignedUserId(String? assignedUserId) =>
      _$this._assignedUserId = assignedUserId;

  bool? _clearAssignedUser;
  bool? get clearAssignedUser => _$this._clearAssignedUser;
  set clearAssignedUser(bool? clearAssignedUser) =>
      _$this._clearAssignedUser = clearAssignedUser;

  ListBuilder<HostSessionUserAssign>? _addSessionUsers;
  ListBuilder<HostSessionUserAssign> get addSessionUsers =>
      _$this._addSessionUsers ??= ListBuilder<HostSessionUserAssign>();
  set addSessionUsers(ListBuilder<HostSessionUserAssign>? addSessionUsers) =>
      _$this._addSessionUsers = addSessionUsers;

  ListBuilder<String>? _revokeSessionUsers;
  ListBuilder<String> get revokeSessionUsers =>
      _$this._revokeSessionUsers ??= ListBuilder<String>();
  set revokeSessionUsers(ListBuilder<String>? revokeSessionUsers) =>
      _$this._revokeSessionUsers = revokeSessionUsers;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostUpdateBuilder() {
    HostUpdate._defaults(this);
  }

  HostUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _assignedUserId = $v.assignedUserId;
      _clearAssignedUser = $v.clearAssignedUser;
      _addSessionUsers = $v.addSessionUsers?.toBuilder();
      _revokeSessionUsers = $v.revokeSessionUsers?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostUpdate other) {
    _$v = other as _$HostUpdate;
  }

  @override
  void update(void Function(HostUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostUpdate build() => _build();

  _$HostUpdate _build() {
    _$HostUpdate _$result;
    try {
      _$result =
          _$v ??
          _$HostUpdate._(
            name: name,
            assignedUserId: assignedUserId,
            clearAssignedUser: clearAssignedUser,
            addSessionUsers: _addSessionUsers?.build(),
            revokeSessionUsers: _revokeSessionUsers?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'HostUpdate',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'addSessionUsers';
        _addSessionUsers?.build();
        _$failedField = 'revokeSessionUsers';
        _revokeSessionUsers?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostUpdate',
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
