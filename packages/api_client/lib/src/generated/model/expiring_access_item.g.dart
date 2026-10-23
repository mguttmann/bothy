// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'expiring_access_item.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ExpiringAccessItemKindEnum _$expiringAccessItemKindEnum_grant =
    const ExpiringAccessItemKindEnum._('grant');
const ExpiringAccessItemKindEnum _$expiringAccessItemKindEnum_membership =
    const ExpiringAccessItemKindEnum._('membership');
const ExpiringAccessItemKindEnum _$expiringAccessItemKindEnum_token =
    const ExpiringAccessItemKindEnum._('token');
const ExpiringAccessItemKindEnum
_$expiringAccessItemKindEnum_unknownDefaultOpenApi =
    const ExpiringAccessItemKindEnum._('unknownDefaultOpenApi');

ExpiringAccessItemKindEnum _$expiringAccessItemKindEnumValueOf(String name) {
  switch (name) {
    case 'grant':
      return _$expiringAccessItemKindEnum_grant;
    case 'membership':
      return _$expiringAccessItemKindEnum_membership;
    case 'token':
      return _$expiringAccessItemKindEnum_token;
    case 'unknownDefaultOpenApi':
      return _$expiringAccessItemKindEnum_unknownDefaultOpenApi;
    default:
      return _$expiringAccessItemKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ExpiringAccessItemKindEnum> _$expiringAccessItemKindEnumValues =
    BuiltSet<ExpiringAccessItemKindEnum>(const <ExpiringAccessItemKindEnum>[
      _$expiringAccessItemKindEnum_grant,
      _$expiringAccessItemKindEnum_membership,
      _$expiringAccessItemKindEnum_token,
      _$expiringAccessItemKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ExpiringAccessItemKindEnum> _$expiringAccessItemKindEnumSerializer =
    _$ExpiringAccessItemKindEnumSerializer();

class _$ExpiringAccessItemKindEnumSerializer
    implements PrimitiveSerializer<ExpiringAccessItemKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'grant': 'grant',
    'membership': 'membership',
    'token': 'token',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'grant': 'grant',
    'membership': 'membership',
    'token': 'token',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ExpiringAccessItemKindEnum];
  @override
  final String wireName = 'ExpiringAccessItemKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ExpiringAccessItemKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ExpiringAccessItemKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ExpiringAccessItemKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ExpiringAccessItem extends ExpiringAccessItem {
  @override
  final ExpiringAccessItemKindEnum kind;
  @override
  final DateTime expiresAt;
  @override
  final Grant? grant;
  @override
  final String? membershipId;
  @override
  final String? groupId;
  @override
  final String? userId;
  @override
  final String? tokenId;
  @override
  final String? principalId;

  factory _$ExpiringAccessItem([
    void Function(ExpiringAccessItemBuilder)? updates,
  ]) => (ExpiringAccessItemBuilder()..update(updates))._build();

  _$ExpiringAccessItem._({
    required this.kind,
    required this.expiresAt,
    this.grant,
    this.membershipId,
    this.groupId,
    this.userId,
    this.tokenId,
    this.principalId,
  }) : super._();
  @override
  ExpiringAccessItem rebuild(
    void Function(ExpiringAccessItemBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ExpiringAccessItemBuilder toBuilder() =>
      ExpiringAccessItemBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ExpiringAccessItem &&
        kind == other.kind &&
        expiresAt == other.expiresAt &&
        grant == other.grant &&
        membershipId == other.membershipId &&
        groupId == other.groupId &&
        userId == other.userId &&
        tokenId == other.tokenId &&
        principalId == other.principalId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, grant.hashCode);
    _$hash = $jc(_$hash, membershipId.hashCode);
    _$hash = $jc(_$hash, groupId.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, tokenId.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ExpiringAccessItem')
          ..add('kind', kind)
          ..add('expiresAt', expiresAt)
          ..add('grant', grant)
          ..add('membershipId', membershipId)
          ..add('groupId', groupId)
          ..add('userId', userId)
          ..add('tokenId', tokenId)
          ..add('principalId', principalId))
        .toString();
  }
}

class ExpiringAccessItemBuilder
    implements Builder<ExpiringAccessItem, ExpiringAccessItemBuilder> {
  _$ExpiringAccessItem? _$v;

  ExpiringAccessItemKindEnum? _kind;
  ExpiringAccessItemKindEnum? get kind => _$this._kind;
  set kind(ExpiringAccessItemKindEnum? kind) => _$this._kind = kind;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  GrantBuilder? _grant;
  GrantBuilder get grant => _$this._grant ??= GrantBuilder();
  set grant(GrantBuilder? grant) => _$this._grant = grant;

  String? _membershipId;
  String? get membershipId => _$this._membershipId;
  set membershipId(String? membershipId) => _$this._membershipId = membershipId;

  String? _groupId;
  String? get groupId => _$this._groupId;
  set groupId(String? groupId) => _$this._groupId = groupId;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _tokenId;
  String? get tokenId => _$this._tokenId;
  set tokenId(String? tokenId) => _$this._tokenId = tokenId;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  ExpiringAccessItemBuilder() {
    ExpiringAccessItem._defaults(this);
  }

  ExpiringAccessItemBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _expiresAt = $v.expiresAt;
      _grant = $v.grant?.toBuilder();
      _membershipId = $v.membershipId;
      _groupId = $v.groupId;
      _userId = $v.userId;
      _tokenId = $v.tokenId;
      _principalId = $v.principalId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ExpiringAccessItem other) {
    _$v = other as _$ExpiringAccessItem;
  }

  @override
  void update(void Function(ExpiringAccessItemBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ExpiringAccessItem build() => _build();

  _$ExpiringAccessItem _build() {
    _$ExpiringAccessItem _$result;
    try {
      _$result =
          _$v ??
          _$ExpiringAccessItem._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'ExpiringAccessItem',
              'kind',
            ),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'ExpiringAccessItem',
              'expiresAt',
            ),
            grant: _grant?.build(),
            membershipId: membershipId,
            groupId: groupId,
            userId: userId,
            tokenId: tokenId,
            principalId: principalId,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'grant';
        _grant?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ExpiringAccessItem',
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
