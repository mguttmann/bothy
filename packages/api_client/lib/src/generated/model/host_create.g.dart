// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostCreate extends HostCreate {
  @override
  final String name;
  @override
  final String? assignedUserId;
  @override
  final String reason;

  factory _$HostCreate([void Function(HostCreateBuilder)? updates]) =>
      (HostCreateBuilder()..update(updates))._build();

  _$HostCreate._({
    required this.name,
    this.assignedUserId,
    required this.reason,
  }) : super._();
  @override
  HostCreate rebuild(void Function(HostCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostCreateBuilder toBuilder() => HostCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostCreate &&
        name == other.name &&
        assignedUserId == other.assignedUserId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, assignedUserId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostCreate')
          ..add('name', name)
          ..add('assignedUserId', assignedUserId)
          ..add('reason', reason))
        .toString();
  }
}

class HostCreateBuilder implements Builder<HostCreate, HostCreateBuilder> {
  _$HostCreate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _assignedUserId;
  String? get assignedUserId => _$this._assignedUserId;
  set assignedUserId(String? assignedUserId) =>
      _$this._assignedUserId = assignedUserId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostCreateBuilder() {
    HostCreate._defaults(this);
  }

  HostCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _assignedUserId = $v.assignedUserId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostCreate other) {
    _$v = other as _$HostCreate;
  }

  @override
  void update(void Function(HostCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostCreate build() => _build();

  _$HostCreate _build() {
    final _$result =
        _$v ??
        _$HostCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'HostCreate',
            'name',
          ),
          assignedUserId: assignedUserId,
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'HostCreate',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
