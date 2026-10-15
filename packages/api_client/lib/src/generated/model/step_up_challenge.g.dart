// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_challenge.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpChallenge extends StepUpChallenge {
  @override
  final String challengeId;
  @override
  final String challenge;
  @override
  final BuiltMap<String, JsonObject?>? passkeyOptions;
  @override
  final DateTime expiresAt;

  factory _$StepUpChallenge([void Function(StepUpChallengeBuilder)? updates]) =>
      (StepUpChallengeBuilder()..update(updates))._build();

  _$StepUpChallenge._({
    required this.challengeId,
    required this.challenge,
    this.passkeyOptions,
    required this.expiresAt,
  }) : super._();
  @override
  StepUpChallenge rebuild(void Function(StepUpChallengeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StepUpChallengeBuilder toBuilder() => StepUpChallengeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpChallenge &&
        challengeId == other.challengeId &&
        challenge == other.challenge &&
        passkeyOptions == other.passkeyOptions &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, challenge.hashCode);
    _$hash = $jc(_$hash, passkeyOptions.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpChallenge')
          ..add('challengeId', challengeId)
          ..add('challenge', challenge)
          ..add('passkeyOptions', passkeyOptions)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class StepUpChallengeBuilder
    implements Builder<StepUpChallenge, StepUpChallengeBuilder> {
  _$StepUpChallenge? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  String? _challenge;
  String? get challenge => _$this._challenge;
  set challenge(String? challenge) => _$this._challenge = challenge;

  MapBuilder<String, JsonObject?>? _passkeyOptions;
  MapBuilder<String, JsonObject?> get passkeyOptions =>
      _$this._passkeyOptions ??= MapBuilder<String, JsonObject?>();
  set passkeyOptions(MapBuilder<String, JsonObject?>? passkeyOptions) =>
      _$this._passkeyOptions = passkeyOptions;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  StepUpChallengeBuilder() {
    StepUpChallenge._defaults(this);
  }

  StepUpChallengeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _challenge = $v.challenge;
      _passkeyOptions = $v.passkeyOptions?.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpChallenge other) {
    _$v = other as _$StepUpChallenge;
  }

  @override
  void update(void Function(StepUpChallengeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpChallenge build() => _build();

  _$StepUpChallenge _build() {
    _$StepUpChallenge _$result;
    try {
      _$result =
          _$v ??
          _$StepUpChallenge._(
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'StepUpChallenge',
              'challengeId',
            ),
            challenge: BuiltValueNullFieldError.checkNotNull(
              challenge,
              r'StepUpChallenge',
              'challenge',
            ),
            passkeyOptions: _passkeyOptions?.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'StepUpChallenge',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'passkeyOptions';
        _passkeyOptions?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StepUpChallenge',
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
