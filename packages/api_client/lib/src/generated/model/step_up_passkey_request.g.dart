// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_passkey_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpPasskeyRequest extends StepUpPasskeyRequest {
  @override
  final String challengeId;
  @override
  final BuiltMap<String, JsonObject?> credential;

  factory _$StepUpPasskeyRequest([
    void Function(StepUpPasskeyRequestBuilder)? updates,
  ]) => (StepUpPasskeyRequestBuilder()..update(updates))._build();

  _$StepUpPasskeyRequest._({
    required this.challengeId,
    required this.credential,
  }) : super._();
  @override
  StepUpPasskeyRequest rebuild(
    void Function(StepUpPasskeyRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StepUpPasskeyRequestBuilder toBuilder() =>
      StepUpPasskeyRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpPasskeyRequest &&
        challengeId == other.challengeId &&
        credential == other.credential;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, credential.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpPasskeyRequest')
          ..add('challengeId', challengeId)
          ..add('credential', credential))
        .toString();
  }
}

class StepUpPasskeyRequestBuilder
    implements Builder<StepUpPasskeyRequest, StepUpPasskeyRequestBuilder> {
  _$StepUpPasskeyRequest? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  MapBuilder<String, JsonObject?>? _credential;
  MapBuilder<String, JsonObject?> get credential =>
      _$this._credential ??= MapBuilder<String, JsonObject?>();
  set credential(MapBuilder<String, JsonObject?>? credential) =>
      _$this._credential = credential;

  StepUpPasskeyRequestBuilder() {
    StepUpPasskeyRequest._defaults(this);
  }

  StepUpPasskeyRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _credential = $v.credential.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpPasskeyRequest other) {
    _$v = other as _$StepUpPasskeyRequest;
  }

  @override
  void update(void Function(StepUpPasskeyRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpPasskeyRequest build() => _build();

  _$StepUpPasskeyRequest _build() {
    _$StepUpPasskeyRequest _$result;
    try {
      _$result =
          _$v ??
          _$StepUpPasskeyRequest._(
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'StepUpPasskeyRequest',
              'challengeId',
            ),
            credential: credential.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'credential';
        credential.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StepUpPasskeyRequest',
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
