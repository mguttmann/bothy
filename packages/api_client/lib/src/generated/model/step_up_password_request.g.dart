// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_password_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpPasswordRequest extends StepUpPasswordRequest {
  @override
  final String challengeId;
  @override
  final String password;
  @override
  final String totpCode;

  factory _$StepUpPasswordRequest([
    void Function(StepUpPasswordRequestBuilder)? updates,
  ]) => (StepUpPasswordRequestBuilder()..update(updates))._build();

  _$StepUpPasswordRequest._({
    required this.challengeId,
    required this.password,
    required this.totpCode,
  }) : super._();
  @override
  StepUpPasswordRequest rebuild(
    void Function(StepUpPasswordRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StepUpPasswordRequestBuilder toBuilder() =>
      StepUpPasswordRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpPasswordRequest &&
        challengeId == other.challengeId &&
        password == other.password &&
        totpCode == other.totpCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, password.hashCode);
    _$hash = $jc(_$hash, totpCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpPasswordRequest')
          ..add('challengeId', challengeId)
          ..add('password', password)
          ..add('totpCode', totpCode))
        .toString();
  }
}

class StepUpPasswordRequestBuilder
    implements Builder<StepUpPasswordRequest, StepUpPasswordRequestBuilder> {
  _$StepUpPasswordRequest? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  String? _password;
  String? get password => _$this._password;
  set password(String? password) => _$this._password = password;

  String? _totpCode;
  String? get totpCode => _$this._totpCode;
  set totpCode(String? totpCode) => _$this._totpCode = totpCode;

  StepUpPasswordRequestBuilder() {
    StepUpPasswordRequest._defaults(this);
  }

  StepUpPasswordRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _password = $v.password;
      _totpCode = $v.totpCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpPasswordRequest other) {
    _$v = other as _$StepUpPasswordRequest;
  }

  @override
  void update(void Function(StepUpPasswordRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpPasswordRequest build() => _build();

  _$StepUpPasswordRequest _build() {
    final _$result =
        _$v ??
        _$StepUpPasswordRequest._(
          challengeId: BuiltValueNullFieldError.checkNotNull(
            challengeId,
            r'StepUpPasswordRequest',
            'challengeId',
          ),
          password: BuiltValueNullFieldError.checkNotNull(
            password,
            r'StepUpPasswordRequest',
            'password',
          ),
          totpCode: BuiltValueNullFieldError.checkNotNull(
            totpCode,
            r'StepUpPasswordRequest',
            'totpCode',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
