// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'enrollment_code.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const EnrollmentCodePurposeEnum _$enrollmentCodePurposeEnum_firstLogin =
    const EnrollmentCodePurposeEnum._('firstLogin');
const EnrollmentCodePurposeEnum _$enrollmentCodePurposeEnum_recovery =
    const EnrollmentCodePurposeEnum._('recovery');
const EnrollmentCodePurposeEnum _$enrollmentCodePurposeEnum_identityLink =
    const EnrollmentCodePurposeEnum._('identityLink');
const EnrollmentCodePurposeEnum
_$enrollmentCodePurposeEnum_unknownDefaultOpenApi =
    const EnrollmentCodePurposeEnum._('unknownDefaultOpenApi');

EnrollmentCodePurposeEnum _$enrollmentCodePurposeEnumValueOf(String name) {
  switch (name) {
    case 'firstLogin':
      return _$enrollmentCodePurposeEnum_firstLogin;
    case 'recovery':
      return _$enrollmentCodePurposeEnum_recovery;
    case 'identityLink':
      return _$enrollmentCodePurposeEnum_identityLink;
    case 'unknownDefaultOpenApi':
      return _$enrollmentCodePurposeEnum_unknownDefaultOpenApi;
    default:
      return _$enrollmentCodePurposeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<EnrollmentCodePurposeEnum> _$enrollmentCodePurposeEnumValues =
    BuiltSet<EnrollmentCodePurposeEnum>(const <EnrollmentCodePurposeEnum>[
      _$enrollmentCodePurposeEnum_firstLogin,
      _$enrollmentCodePurposeEnum_recovery,
      _$enrollmentCodePurposeEnum_identityLink,
      _$enrollmentCodePurposeEnum_unknownDefaultOpenApi,
    ]);

Serializer<EnrollmentCodePurposeEnum> _$enrollmentCodePurposeEnumSerializer =
    _$EnrollmentCodePurposeEnumSerializer();

class _$EnrollmentCodePurposeEnumSerializer
    implements PrimitiveSerializer<EnrollmentCodePurposeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'firstLogin': 'first_login',
    'recovery': 'recovery',
    'identityLink': 'identity_link',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'first_login': 'firstLogin',
    'recovery': 'recovery',
    'identity_link': 'identityLink',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[EnrollmentCodePurposeEnum];
  @override
  final String wireName = 'EnrollmentCodePurposeEnum';

  @override
  Object serialize(
    Serializers serializers,
    EnrollmentCodePurposeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  EnrollmentCodePurposeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => EnrollmentCodePurposeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$EnrollmentCode extends EnrollmentCode {
  @override
  final String code;
  @override
  final EnrollmentCodePurposeEnum purpose;
  @override
  final DateTime expiresAt;

  factory _$EnrollmentCode([void Function(EnrollmentCodeBuilder)? updates]) =>
      (EnrollmentCodeBuilder()..update(updates))._build();

  _$EnrollmentCode._({
    required this.code,
    required this.purpose,
    required this.expiresAt,
  }) : super._();
  @override
  EnrollmentCode rebuild(void Function(EnrollmentCodeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EnrollmentCodeBuilder toBuilder() => EnrollmentCodeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EnrollmentCode &&
        code == other.code &&
        purpose == other.purpose &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EnrollmentCode')
          ..add('code', code)
          ..add('purpose', purpose)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class EnrollmentCodeBuilder
    implements Builder<EnrollmentCode, EnrollmentCodeBuilder> {
  _$EnrollmentCode? _$v;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  EnrollmentCodePurposeEnum? _purpose;
  EnrollmentCodePurposeEnum? get purpose => _$this._purpose;
  set purpose(EnrollmentCodePurposeEnum? purpose) => _$this._purpose = purpose;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  EnrollmentCodeBuilder() {
    EnrollmentCode._defaults(this);
  }

  EnrollmentCodeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _code = $v.code;
      _purpose = $v.purpose;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EnrollmentCode other) {
    _$v = other as _$EnrollmentCode;
  }

  @override
  void update(void Function(EnrollmentCodeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EnrollmentCode build() => _build();

  _$EnrollmentCode _build() {
    final _$result =
        _$v ??
        _$EnrollmentCode._(
          code: BuiltValueNullFieldError.checkNotNull(
            code,
            r'EnrollmentCode',
            'code',
          ),
          purpose: BuiltValueNullFieldError.checkNotNull(
            purpose,
            r'EnrollmentCode',
            'purpose',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'EnrollmentCode',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
