// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'web_authn_challenge.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebAuthnChallenge extends WebAuthnChallenge {
  @override
  final String challengeId;
  @override
  final BuiltMap<String, JsonObject?> publicKey;
  @override
  final DateTime expiresAt;

  factory _$WebAuthnChallenge([
    void Function(WebAuthnChallengeBuilder)? updates,
  ]) => (WebAuthnChallengeBuilder()..update(updates))._build();

  _$WebAuthnChallenge._({
    required this.challengeId,
    required this.publicKey,
    required this.expiresAt,
  }) : super._();
  @override
  WebAuthnChallenge rebuild(void Function(WebAuthnChallengeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  WebAuthnChallengeBuilder toBuilder() =>
      WebAuthnChallengeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebAuthnChallenge &&
        challengeId == other.challengeId &&
        publicKey == other.publicKey &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, publicKey.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WebAuthnChallenge')
          ..add('challengeId', challengeId)
          ..add('publicKey', publicKey)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class WebAuthnChallengeBuilder
    implements Builder<WebAuthnChallenge, WebAuthnChallengeBuilder> {
  _$WebAuthnChallenge? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  MapBuilder<String, JsonObject?>? _publicKey;
  MapBuilder<String, JsonObject?> get publicKey =>
      _$this._publicKey ??= MapBuilder<String, JsonObject?>();
  set publicKey(MapBuilder<String, JsonObject?>? publicKey) =>
      _$this._publicKey = publicKey;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  WebAuthnChallengeBuilder() {
    WebAuthnChallenge._defaults(this);
  }

  WebAuthnChallengeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _publicKey = $v.publicKey.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebAuthnChallenge other) {
    _$v = other as _$WebAuthnChallenge;
  }

  @override
  void update(void Function(WebAuthnChallengeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebAuthnChallenge build() => _build();

  _$WebAuthnChallenge _build() {
    _$WebAuthnChallenge _$result;
    try {
      _$result =
          _$v ??
          _$WebAuthnChallenge._(
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'WebAuthnChallenge',
              'challengeId',
            ),
            publicKey: publicKey.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'WebAuthnChallenge',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'publicKey';
        publicKey.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebAuthnChallenge',
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
