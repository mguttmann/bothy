// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'enroll_complete_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EnrollCompleteRequest extends EnrollCompleteRequest {
  @override
  final String challengeId;
  @override
  final BuiltMap<String, JsonObject?> credential;
  @override
  final String passkeyName;
  @override
  final DeviceRegistration device;

  factory _$EnrollCompleteRequest([
    void Function(EnrollCompleteRequestBuilder)? updates,
  ]) => (EnrollCompleteRequestBuilder()..update(updates))._build();

  _$EnrollCompleteRequest._({
    required this.challengeId,
    required this.credential,
    required this.passkeyName,
    required this.device,
  }) : super._();
  @override
  EnrollCompleteRequest rebuild(
    void Function(EnrollCompleteRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  EnrollCompleteRequestBuilder toBuilder() =>
      EnrollCompleteRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EnrollCompleteRequest &&
        challengeId == other.challengeId &&
        credential == other.credential &&
        passkeyName == other.passkeyName &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, credential.hashCode);
    _$hash = $jc(_$hash, passkeyName.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EnrollCompleteRequest')
          ..add('challengeId', challengeId)
          ..add('credential', credential)
          ..add('passkeyName', passkeyName)
          ..add('device', device))
        .toString();
  }
}

class EnrollCompleteRequestBuilder
    implements Builder<EnrollCompleteRequest, EnrollCompleteRequestBuilder> {
  _$EnrollCompleteRequest? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  MapBuilder<String, JsonObject?>? _credential;
  MapBuilder<String, JsonObject?> get credential =>
      _$this._credential ??= MapBuilder<String, JsonObject?>();
  set credential(MapBuilder<String, JsonObject?>? credential) =>
      _$this._credential = credential;

  String? _passkeyName;
  String? get passkeyName => _$this._passkeyName;
  set passkeyName(String? passkeyName) => _$this._passkeyName = passkeyName;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  EnrollCompleteRequestBuilder() {
    EnrollCompleteRequest._defaults(this);
  }

  EnrollCompleteRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _credential = $v.credential.toBuilder();
      _passkeyName = $v.passkeyName;
      _device = $v.device.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EnrollCompleteRequest other) {
    _$v = other as _$EnrollCompleteRequest;
  }

  @override
  void update(void Function(EnrollCompleteRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EnrollCompleteRequest build() => _build();

  _$EnrollCompleteRequest _build() {
    _$EnrollCompleteRequest _$result;
    try {
      _$result =
          _$v ??
          _$EnrollCompleteRequest._(
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'EnrollCompleteRequest',
              'challengeId',
            ),
            credential: credential.build(),
            passkeyName: BuiltValueNullFieldError.checkNotNull(
              passkeyName,
              r'EnrollCompleteRequest',
              'passkeyName',
            ),
            device: device.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'credential';
        credential.build();

        _$failedField = 'device';
        device.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'EnrollCompleteRequest',
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
