// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GroupChangedPayloadChangeEnum _$groupChangedPayloadChangeEnum_created =
    const GroupChangedPayloadChangeEnum._('created');
const GroupChangedPayloadChangeEnum _$groupChangedPayloadChangeEnum_updated =
    const GroupChangedPayloadChangeEnum._('updated');
const GroupChangedPayloadChangeEnum _$groupChangedPayloadChangeEnum_deleted =
    const GroupChangedPayloadChangeEnum._('deleted');
const GroupChangedPayloadChangeEnum
_$groupChangedPayloadChangeEnum_memberAdded =
    const GroupChangedPayloadChangeEnum._('memberAdded');
const GroupChangedPayloadChangeEnum
_$groupChangedPayloadChangeEnum_memberRemoved =
    const GroupChangedPayloadChangeEnum._('memberRemoved');
const GroupChangedPayloadChangeEnum
_$groupChangedPayloadChangeEnum_memberExpired =
    const GroupChangedPayloadChangeEnum._('memberExpired');
const GroupChangedPayloadChangeEnum
_$groupChangedPayloadChangeEnum_unknownDefaultOpenApi =
    const GroupChangedPayloadChangeEnum._('unknownDefaultOpenApi');

GroupChangedPayloadChangeEnum _$groupChangedPayloadChangeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'created':
      return _$groupChangedPayloadChangeEnum_created;
    case 'updated':
      return _$groupChangedPayloadChangeEnum_updated;
    case 'deleted':
      return _$groupChangedPayloadChangeEnum_deleted;
    case 'memberAdded':
      return _$groupChangedPayloadChangeEnum_memberAdded;
    case 'memberRemoved':
      return _$groupChangedPayloadChangeEnum_memberRemoved;
    case 'memberExpired':
      return _$groupChangedPayloadChangeEnum_memberExpired;
    case 'unknownDefaultOpenApi':
      return _$groupChangedPayloadChangeEnum_unknownDefaultOpenApi;
    default:
      return _$groupChangedPayloadChangeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<GroupChangedPayloadChangeEnum>
_$groupChangedPayloadChangeEnumValues = BuiltSet<GroupChangedPayloadChangeEnum>(
  const <GroupChangedPayloadChangeEnum>[
    _$groupChangedPayloadChangeEnum_created,
    _$groupChangedPayloadChangeEnum_updated,
    _$groupChangedPayloadChangeEnum_deleted,
    _$groupChangedPayloadChangeEnum_memberAdded,
    _$groupChangedPayloadChangeEnum_memberRemoved,
    _$groupChangedPayloadChangeEnum_memberExpired,
    _$groupChangedPayloadChangeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<GroupChangedPayloadChangeEnum>
_$groupChangedPayloadChangeEnumSerializer =
    _$GroupChangedPayloadChangeEnumSerializer();

class _$GroupChangedPayloadChangeEnumSerializer
    implements PrimitiveSerializer<GroupChangedPayloadChangeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'created': 'created',
    'updated': 'updated',
    'deleted': 'deleted',
    'memberAdded': 'member_added',
    'memberRemoved': 'member_removed',
    'memberExpired': 'member_expired',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'created': 'created',
    'updated': 'updated',
    'deleted': 'deleted',
    'member_added': 'memberAdded',
    'member_removed': 'memberRemoved',
    'member_expired': 'memberExpired',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GroupChangedPayloadChangeEnum];
  @override
  final String wireName = 'GroupChangedPayloadChangeEnum';

  @override
  Object serialize(
    Serializers serializers,
    GroupChangedPayloadChangeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GroupChangedPayloadChangeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GroupChangedPayloadChangeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$GroupChangedPayload extends GroupChangedPayload {
  @override
  final Group group;
  @override
  final GroupChangedPayloadChangeEnum change;
  @override
  final String? userId;

  factory _$GroupChangedPayload([
    void Function(GroupChangedPayloadBuilder)? updates,
  ]) => (GroupChangedPayloadBuilder()..update(updates))._build();

  _$GroupChangedPayload._({
    required this.group,
    required this.change,
    this.userId,
  }) : super._();
  @override
  GroupChangedPayload rebuild(
    void Function(GroupChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GroupChangedPayloadBuilder toBuilder() =>
      GroupChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupChangedPayload &&
        group == other.group &&
        change == other.change &&
        userId == other.userId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, group.hashCode);
    _$hash = $jc(_$hash, change.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GroupChangedPayload')
          ..add('group', group)
          ..add('change', change)
          ..add('userId', userId))
        .toString();
  }
}

class GroupChangedPayloadBuilder
    implements Builder<GroupChangedPayload, GroupChangedPayloadBuilder> {
  _$GroupChangedPayload? _$v;

  GroupBuilder? _group;
  GroupBuilder get group => _$this._group ??= GroupBuilder();
  set group(GroupBuilder? group) => _$this._group = group;

  GroupChangedPayloadChangeEnum? _change;
  GroupChangedPayloadChangeEnum? get change => _$this._change;
  set change(GroupChangedPayloadChangeEnum? change) => _$this._change = change;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  GroupChangedPayloadBuilder() {
    GroupChangedPayload._defaults(this);
  }

  GroupChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _group = $v.group.toBuilder();
      _change = $v.change;
      _userId = $v.userId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupChangedPayload other) {
    _$v = other as _$GroupChangedPayload;
  }

  @override
  void update(void Function(GroupChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupChangedPayload build() => _build();

  _$GroupChangedPayload _build() {
    _$GroupChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$GroupChangedPayload._(
            group: group.build(),
            change: BuiltValueNullFieldError.checkNotNull(
              change,
              r'GroupChangedPayload',
              'change',
            ),
            userId: userId,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'group';
        group.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GroupChangedPayload',
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
