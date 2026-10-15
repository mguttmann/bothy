// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'signing_key_rotation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SigningKeyRotation extends SigningKeyRotation {
  @override
  final SigningKeyPurpose purpose;
  @override
  final String newKid;
  @override
  final String previousKid;
  @override
  final DateTime previousNotAfter;

  factory _$SigningKeyRotation([
    void Function(SigningKeyRotationBuilder)? updates,
  ]) => (SigningKeyRotationBuilder()..update(updates))._build();

  _$SigningKeyRotation._({
    required this.purpose,
    required this.newKid,
    required this.previousKid,
    required this.previousNotAfter,
  }) : super._();
  @override
  SigningKeyRotation rebuild(
    void Function(SigningKeyRotationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SigningKeyRotationBuilder toBuilder() =>
      SigningKeyRotationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SigningKeyRotation &&
        purpose == other.purpose &&
        newKid == other.newKid &&
        previousKid == other.previousKid &&
        previousNotAfter == other.previousNotAfter;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jc(_$hash, newKid.hashCode);
    _$hash = $jc(_$hash, previousKid.hashCode);
    _$hash = $jc(_$hash, previousNotAfter.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SigningKeyRotation')
          ..add('purpose', purpose)
          ..add('newKid', newKid)
          ..add('previousKid', previousKid)
          ..add('previousNotAfter', previousNotAfter))
        .toString();
  }
}

class SigningKeyRotationBuilder
    implements Builder<SigningKeyRotation, SigningKeyRotationBuilder> {
  _$SigningKeyRotation? _$v;

  SigningKeyPurpose? _purpose;
  SigningKeyPurpose? get purpose => _$this._purpose;
  set purpose(SigningKeyPurpose? purpose) => _$this._purpose = purpose;

  String? _newKid;
  String? get newKid => _$this._newKid;
  set newKid(String? newKid) => _$this._newKid = newKid;

  String? _previousKid;
  String? get previousKid => _$this._previousKid;
  set previousKid(String? previousKid) => _$this._previousKid = previousKid;

  DateTime? _previousNotAfter;
  DateTime? get previousNotAfter => _$this._previousNotAfter;
  set previousNotAfter(DateTime? previousNotAfter) =>
      _$this._previousNotAfter = previousNotAfter;

  SigningKeyRotationBuilder() {
    SigningKeyRotation._defaults(this);
  }

  SigningKeyRotationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _purpose = $v.purpose;
      _newKid = $v.newKid;
      _previousKid = $v.previousKid;
      _previousNotAfter = $v.previousNotAfter;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SigningKeyRotation other) {
    _$v = other as _$SigningKeyRotation;
  }

  @override
  void update(void Function(SigningKeyRotationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SigningKeyRotation build() => _build();

  _$SigningKeyRotation _build() {
    final _$result =
        _$v ??
        _$SigningKeyRotation._(
          purpose: BuiltValueNullFieldError.checkNotNull(
            purpose,
            r'SigningKeyRotation',
            'purpose',
          ),
          newKid: BuiltValueNullFieldError.checkNotNull(
            newKid,
            r'SigningKeyRotation',
            'newKid',
          ),
          previousKid: BuiltValueNullFieldError.checkNotNull(
            previousKid,
            r'SigningKeyRotation',
            'previousKid',
          ),
          previousNotAfter: BuiltValueNullFieldError.checkNotNull(
            previousNotAfter,
            r'SigningKeyRotation',
            'previousNotAfter',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
