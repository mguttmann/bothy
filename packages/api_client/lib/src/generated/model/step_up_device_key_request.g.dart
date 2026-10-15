// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_device_key_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpDeviceKeyRequest extends StepUpDeviceKeyRequest {
  @override
  final String challengeId;
  @override
  final String signature;

  factory _$StepUpDeviceKeyRequest([
    void Function(StepUpDeviceKeyRequestBuilder)? updates,
  ]) => (StepUpDeviceKeyRequestBuilder()..update(updates))._build();

  _$StepUpDeviceKeyRequest._({
    required this.challengeId,
    required this.signature,
  }) : super._();
  @override
  StepUpDeviceKeyRequest rebuild(
    void Function(StepUpDeviceKeyRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StepUpDeviceKeyRequestBuilder toBuilder() =>
      StepUpDeviceKeyRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpDeviceKeyRequest &&
        challengeId == other.challengeId &&
        signature == other.signature;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, signature.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpDeviceKeyRequest')
          ..add('challengeId', challengeId)
          ..add('signature', signature))
        .toString();
  }
}

class StepUpDeviceKeyRequestBuilder
    implements Builder<StepUpDeviceKeyRequest, StepUpDeviceKeyRequestBuilder> {
  _$StepUpDeviceKeyRequest? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  String? _signature;
  String? get signature => _$this._signature;
  set signature(String? signature) => _$this._signature = signature;

  StepUpDeviceKeyRequestBuilder() {
    StepUpDeviceKeyRequest._defaults(this);
  }

  StepUpDeviceKeyRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _signature = $v.signature;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpDeviceKeyRequest other) {
    _$v = other as _$StepUpDeviceKeyRequest;
  }

  @override
  void update(void Function(StepUpDeviceKeyRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpDeviceKeyRequest build() => _build();

  _$StepUpDeviceKeyRequest _build() {
    final _$result =
        _$v ??
        _$StepUpDeviceKeyRequest._(
          challengeId: BuiltValueNullFieldError.checkNotNull(
            challengeId,
            r'StepUpDeviceKeyRequest',
            'challengeId',
          ),
          signature: BuiltValueNullFieldError.checkNotNull(
            signature,
            r'StepUpDeviceKeyRequest',
            'signature',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
