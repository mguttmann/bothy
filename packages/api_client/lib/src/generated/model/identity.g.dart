// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Identity extends Identity {
  @override
  final String id;
  @override
  final String providerId;
  @override
  final IdentityProviderKind providerKind;
  @override
  final String subject;
  @override
  final String? issuer;
  @override
  final String? emailAtLink;
  @override
  final DateTime linkedAt;
  @override
  final DateTime? lastLoginAt;

  factory _$Identity([void Function(IdentityBuilder)? updates]) =>
      (IdentityBuilder()..update(updates))._build();

  _$Identity._({
    required this.id,
    required this.providerId,
    required this.providerKind,
    required this.subject,
    this.issuer,
    this.emailAtLink,
    required this.linkedAt,
    this.lastLoginAt,
  }) : super._();
  @override
  Identity rebuild(void Function(IdentityBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  IdentityBuilder toBuilder() => IdentityBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Identity &&
        id == other.id &&
        providerId == other.providerId &&
        providerKind == other.providerKind &&
        subject == other.subject &&
        issuer == other.issuer &&
        emailAtLink == other.emailAtLink &&
        linkedAt == other.linkedAt &&
        lastLoginAt == other.lastLoginAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, providerKind.hashCode);
    _$hash = $jc(_$hash, subject.hashCode);
    _$hash = $jc(_$hash, issuer.hashCode);
    _$hash = $jc(_$hash, emailAtLink.hashCode);
    _$hash = $jc(_$hash, linkedAt.hashCode);
    _$hash = $jc(_$hash, lastLoginAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Identity')
          ..add('id', id)
          ..add('providerId', providerId)
          ..add('providerKind', providerKind)
          ..add('subject', subject)
          ..add('issuer', issuer)
          ..add('emailAtLink', emailAtLink)
          ..add('linkedAt', linkedAt)
          ..add('lastLoginAt', lastLoginAt))
        .toString();
  }
}

class IdentityBuilder implements Builder<Identity, IdentityBuilder> {
  _$Identity? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _providerId;
  String? get providerId => _$this._providerId;
  set providerId(String? providerId) => _$this._providerId = providerId;

  IdentityProviderKind? _providerKind;
  IdentityProviderKind? get providerKind => _$this._providerKind;
  set providerKind(IdentityProviderKind? providerKind) =>
      _$this._providerKind = providerKind;

  String? _subject;
  String? get subject => _$this._subject;
  set subject(String? subject) => _$this._subject = subject;

  String? _issuer;
  String? get issuer => _$this._issuer;
  set issuer(String? issuer) => _$this._issuer = issuer;

  String? _emailAtLink;
  String? get emailAtLink => _$this._emailAtLink;
  set emailAtLink(String? emailAtLink) => _$this._emailAtLink = emailAtLink;

  DateTime? _linkedAt;
  DateTime? get linkedAt => _$this._linkedAt;
  set linkedAt(DateTime? linkedAt) => _$this._linkedAt = linkedAt;

  DateTime? _lastLoginAt;
  DateTime? get lastLoginAt => _$this._lastLoginAt;
  set lastLoginAt(DateTime? lastLoginAt) => _$this._lastLoginAt = lastLoginAt;

  IdentityBuilder() {
    Identity._defaults(this);
  }

  IdentityBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _providerId = $v.providerId;
      _providerKind = $v.providerKind;
      _subject = $v.subject;
      _issuer = $v.issuer;
      _emailAtLink = $v.emailAtLink;
      _linkedAt = $v.linkedAt;
      _lastLoginAt = $v.lastLoginAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Identity other) {
    _$v = other as _$Identity;
  }

  @override
  void update(void Function(IdentityBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Identity build() => _build();

  _$Identity _build() {
    final _$result =
        _$v ??
        _$Identity._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Identity', 'id'),
          providerId: BuiltValueNullFieldError.checkNotNull(
            providerId,
            r'Identity',
            'providerId',
          ),
          providerKind: BuiltValueNullFieldError.checkNotNull(
            providerKind,
            r'Identity',
            'providerKind',
          ),
          subject: BuiltValueNullFieldError.checkNotNull(
            subject,
            r'Identity',
            'subject',
          ),
          issuer: issuer,
          emailAtLink: emailAtLink,
          linkedAt: BuiltValueNullFieldError.checkNotNull(
            linkedAt,
            r'Identity',
            'linkedAt',
          ),
          lastLoginAt: lastLoginAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
