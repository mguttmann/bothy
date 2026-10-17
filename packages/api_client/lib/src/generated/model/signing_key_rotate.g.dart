// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'signing_key_rotate.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SigningKeyRotateModeEnum _$signingKeyRotateModeEnum_scheduled =
    const SigningKeyRotateModeEnum._('scheduled');
const SigningKeyRotateModeEnum _$signingKeyRotateModeEnum_emergency =
    const SigningKeyRotateModeEnum._('emergency');
const SigningKeyRotateModeEnum
_$signingKeyRotateModeEnum_unknownDefaultOpenApi =
    const SigningKeyRotateModeEnum._('unknownDefaultOpenApi');

SigningKeyRotateModeEnum _$signingKeyRotateModeEnumValueOf(String name) {
  switch (name) {
    case 'scheduled':
      return _$signingKeyRotateModeEnum_scheduled;
    case 'emergency':
      return _$signingKeyRotateModeEnum_emergency;
    case 'unknownDefaultOpenApi':
      return _$signingKeyRotateModeEnum_unknownDefaultOpenApi;
    default:
      return _$signingKeyRotateModeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SigningKeyRotateModeEnum> _$signingKeyRotateModeEnumValues =
    BuiltSet<SigningKeyRotateModeEnum>(const <SigningKeyRotateModeEnum>[
      _$signingKeyRotateModeEnum_scheduled,
      _$signingKeyRotateModeEnum_emergency,
      _$signingKeyRotateModeEnum_unknownDefaultOpenApi,
    ]);

Serializer<SigningKeyRotateModeEnum> _$signingKeyRotateModeEnumSerializer =
    _$SigningKeyRotateModeEnumSerializer();

class _$SigningKeyRotateModeEnumSerializer
    implements PrimitiveSerializer<SigningKeyRotateModeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'scheduled': 'scheduled',
    'emergency': 'emergency',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'scheduled': 'scheduled',
    'emergency': 'emergency',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SigningKeyRotateModeEnum];
  @override
  final String wireName = 'SigningKeyRotateModeEnum';

  @override
  Object serialize(
    Serializers serializers,
    SigningKeyRotateModeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SigningKeyRotateModeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SigningKeyRotateModeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SigningKeyRotate extends SigningKeyRotate {
  @override
  final SigningKeyPurpose purpose;
  @override
  final SigningKeyRotateModeEnum mode;
  @override
  final String reason;

  factory _$SigningKeyRotate([
    void Function(SigningKeyRotateBuilder)? updates,
  ]) => (SigningKeyRotateBuilder()..update(updates))._build();

  _$SigningKeyRotate._({
    required this.purpose,
    required this.mode,
    required this.reason,
  }) : super._();
  @override
  SigningKeyRotate rebuild(void Function(SigningKeyRotateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SigningKeyRotateBuilder toBuilder() =>
      SigningKeyRotateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SigningKeyRotate &&
        purpose == other.purpose &&
        mode == other.mode &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SigningKeyRotate')
          ..add('purpose', purpose)
          ..add('mode', mode)
          ..add('reason', reason))
        .toString();
  }
}

class SigningKeyRotateBuilder
    implements Builder<SigningKeyRotate, SigningKeyRotateBuilder> {
  _$SigningKeyRotate? _$v;

  SigningKeyPurpose? _purpose;
  SigningKeyPurpose? get purpose => _$this._purpose;
  set purpose(SigningKeyPurpose? purpose) => _$this._purpose = purpose;

  SigningKeyRotateModeEnum? _mode;
  SigningKeyRotateModeEnum? get mode => _$this._mode;
  set mode(SigningKeyRotateModeEnum? mode) => _$this._mode = mode;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  SigningKeyRotateBuilder() {
    SigningKeyRotate._defaults(this);
  }

  SigningKeyRotateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _purpose = $v.purpose;
      _mode = $v.mode;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SigningKeyRotate other) {
    _$v = other as _$SigningKeyRotate;
  }

  @override
  void update(void Function(SigningKeyRotateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SigningKeyRotate build() => _build();

  _$SigningKeyRotate _build() {
    final _$result =
        _$v ??
        _$SigningKeyRotate._(
          purpose: BuiltValueNullFieldError.checkNotNull(
            purpose,
            r'SigningKeyRotate',
            'purpose',
          ),
          mode: BuiltValueNullFieldError.checkNotNull(
            mode,
            r'SigningKeyRotate',
            'mode',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'SigningKeyRotate',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
