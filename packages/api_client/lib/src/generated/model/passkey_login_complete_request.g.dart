// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'passkey_login_complete_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PasskeyLoginCompleteRequest extends PasskeyLoginCompleteRequest {
  @override
  final LoginClient? client;
  @override
  final String challengeId;
  @override
  final BuiltMap<String, JsonObject?> credential;
  @override
  final DeviceRegistration? device;

  factory _$PasskeyLoginCompleteRequest([
    void Function(PasskeyLoginCompleteRequestBuilder)? updates,
  ]) => (PasskeyLoginCompleteRequestBuilder()..update(updates))._build();

  _$PasskeyLoginCompleteRequest._({
    this.client,
    required this.challengeId,
    required this.credential,
    this.device,
  }) : super._();
  @override
  PasskeyLoginCompleteRequest rebuild(
    void Function(PasskeyLoginCompleteRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PasskeyLoginCompleteRequestBuilder toBuilder() =>
      PasskeyLoginCompleteRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PasskeyLoginCompleteRequest &&
        client == other.client &&
        challengeId == other.challengeId &&
        credential == other.credential &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, client.hashCode);
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, credential.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PasskeyLoginCompleteRequest')
          ..add('client', client)
          ..add('challengeId', challengeId)
          ..add('credential', credential)
          ..add('device', device))
        .toString();
  }
}

class PasskeyLoginCompleteRequestBuilder
    implements
        Builder<
          PasskeyLoginCompleteRequest,
          PasskeyLoginCompleteRequestBuilder
        > {
  _$PasskeyLoginCompleteRequest? _$v;

  LoginClient? _client;
  LoginClient? get client => _$this._client;
  set client(LoginClient? client) => _$this._client = client;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  MapBuilder<String, JsonObject?>? _credential;
  MapBuilder<String, JsonObject?> get credential =>
      _$this._credential ??= MapBuilder<String, JsonObject?>();
  set credential(MapBuilder<String, JsonObject?>? credential) =>
      _$this._credential = credential;

  DeviceRegistrationBuilder? _device;
  DeviceRegistrationBuilder get device =>
      _$this._device ??= DeviceRegistrationBuilder();
  set device(DeviceRegistrationBuilder? device) => _$this._device = device;

  PasskeyLoginCompleteRequestBuilder() {
    PasskeyLoginCompleteRequest._defaults(this);
  }

  PasskeyLoginCompleteRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _client = $v.client;
      _challengeId = $v.challengeId;
      _credential = $v.credential.toBuilder();
      _device = $v.device?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PasskeyLoginCompleteRequest other) {
    _$v = other as _$PasskeyLoginCompleteRequest;
  }

  @override
  void update(void Function(PasskeyLoginCompleteRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PasskeyLoginCompleteRequest build() => _build();

  _$PasskeyLoginCompleteRequest _build() {
    _$PasskeyLoginCompleteRequest _$result;
    try {
      _$result =
          _$v ??
          _$PasskeyLoginCompleteRequest._(
            client: client,
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'PasskeyLoginCompleteRequest',
              'challengeId',
            ),
            credential: credential.build(),
            device: _device?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'credential';
        credential.build();
        _$failedField = 'device';
        _device?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PasskeyLoginCompleteRequest',
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
