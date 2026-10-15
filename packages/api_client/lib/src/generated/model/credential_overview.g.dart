// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'credential_overview.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CredentialOverview extends CredentialOverview {
  @override
  final BuiltList<Identity> identities;
  @override
  final BuiltList<Passkey> passkeys;
  @override
  final bool localPasswordSet;
  @override
  final bool totpEnrolled;

  factory _$CredentialOverview([
    void Function(CredentialOverviewBuilder)? updates,
  ]) => (CredentialOverviewBuilder()..update(updates))._build();

  _$CredentialOverview._({
    required this.identities,
    required this.passkeys,
    required this.localPasswordSet,
    required this.totpEnrolled,
  }) : super._();
  @override
  CredentialOverview rebuild(
    void Function(CredentialOverviewBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CredentialOverviewBuilder toBuilder() =>
      CredentialOverviewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CredentialOverview &&
        identities == other.identities &&
        passkeys == other.passkeys &&
        localPasswordSet == other.localPasswordSet &&
        totpEnrolled == other.totpEnrolled;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, identities.hashCode);
    _$hash = $jc(_$hash, passkeys.hashCode);
    _$hash = $jc(_$hash, localPasswordSet.hashCode);
    _$hash = $jc(_$hash, totpEnrolled.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CredentialOverview')
          ..add('identities', identities)
          ..add('passkeys', passkeys)
          ..add('localPasswordSet', localPasswordSet)
          ..add('totpEnrolled', totpEnrolled))
        .toString();
  }
}

class CredentialOverviewBuilder
    implements Builder<CredentialOverview, CredentialOverviewBuilder> {
  _$CredentialOverview? _$v;

  ListBuilder<Identity>? _identities;
  ListBuilder<Identity> get identities =>
      _$this._identities ??= ListBuilder<Identity>();
  set identities(ListBuilder<Identity>? identities) =>
      _$this._identities = identities;

  ListBuilder<Passkey>? _passkeys;
  ListBuilder<Passkey> get passkeys =>
      _$this._passkeys ??= ListBuilder<Passkey>();
  set passkeys(ListBuilder<Passkey>? passkeys) => _$this._passkeys = passkeys;

  bool? _localPasswordSet;
  bool? get localPasswordSet => _$this._localPasswordSet;
  set localPasswordSet(bool? localPasswordSet) =>
      _$this._localPasswordSet = localPasswordSet;

  bool? _totpEnrolled;
  bool? get totpEnrolled => _$this._totpEnrolled;
  set totpEnrolled(bool? totpEnrolled) => _$this._totpEnrolled = totpEnrolled;

  CredentialOverviewBuilder() {
    CredentialOverview._defaults(this);
  }

  CredentialOverviewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _identities = $v.identities.toBuilder();
      _passkeys = $v.passkeys.toBuilder();
      _localPasswordSet = $v.localPasswordSet;
      _totpEnrolled = $v.totpEnrolled;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CredentialOverview other) {
    _$v = other as _$CredentialOverview;
  }

  @override
  void update(void Function(CredentialOverviewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CredentialOverview build() => _build();

  _$CredentialOverview _build() {
    _$CredentialOverview _$result;
    try {
      _$result =
          _$v ??
          _$CredentialOverview._(
            identities: identities.build(),
            passkeys: passkeys.build(),
            localPasswordSet: BuiltValueNullFieldError.checkNotNull(
              localPasswordSet,
              r'CredentialOverview',
              'localPasswordSet',
            ),
            totpEnrolled: BuiltValueNullFieldError.checkNotNull(
              totpEnrolled,
              r'CredentialOverview',
              'totpEnrolled',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'identities';
        identities.build();
        _$failedField = 'passkeys';
        passkeys.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CredentialOverview',
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
