// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_evidence.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnum_apiKey =
    const BillingEvidenceKeyTypeEnum._('apiKey');
const BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnum_oauth =
    const BillingEvidenceKeyTypeEnum._('oauth');
const BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnum_cloud =
    const BillingEvidenceKeyTypeEnum._('cloud');
const BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnum_none =
    const BillingEvidenceKeyTypeEnum._('none');
const BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnum_unknown =
    const BillingEvidenceKeyTypeEnum._('unknown');
const BillingEvidenceKeyTypeEnum
_$billingEvidenceKeyTypeEnum_unknownDefaultOpenApi =
    const BillingEvidenceKeyTypeEnum._('unknownDefaultOpenApi');

BillingEvidenceKeyTypeEnum _$billingEvidenceKeyTypeEnumValueOf(String name) {
  switch (name) {
    case 'apiKey':
      return _$billingEvidenceKeyTypeEnum_apiKey;
    case 'oauth':
      return _$billingEvidenceKeyTypeEnum_oauth;
    case 'cloud':
      return _$billingEvidenceKeyTypeEnum_cloud;
    case 'none':
      return _$billingEvidenceKeyTypeEnum_none;
    case 'unknown':
      return _$billingEvidenceKeyTypeEnum_unknown;
    case 'unknownDefaultOpenApi':
      return _$billingEvidenceKeyTypeEnum_unknownDefaultOpenApi;
    default:
      return _$billingEvidenceKeyTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<BillingEvidenceKeyTypeEnum> _$billingEvidenceKeyTypeEnumValues =
    BuiltSet<BillingEvidenceKeyTypeEnum>(const <BillingEvidenceKeyTypeEnum>[
      _$billingEvidenceKeyTypeEnum_apiKey,
      _$billingEvidenceKeyTypeEnum_oauth,
      _$billingEvidenceKeyTypeEnum_cloud,
      _$billingEvidenceKeyTypeEnum_none,
      _$billingEvidenceKeyTypeEnum_unknown,
      _$billingEvidenceKeyTypeEnum_unknownDefaultOpenApi,
    ]);

Serializer<BillingEvidenceKeyTypeEnum> _$billingEvidenceKeyTypeEnumSerializer =
    _$BillingEvidenceKeyTypeEnumSerializer();

class _$BillingEvidenceKeyTypeEnumSerializer
    implements PrimitiveSerializer<BillingEvidenceKeyTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'apiKey': 'api_key',
    'oauth': 'oauth',
    'cloud': 'cloud',
    'none': 'none',
    'unknown': 'unknown',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'api_key': 'apiKey',
    'oauth': 'oauth',
    'cloud': 'cloud',
    'none': 'none',
    'unknown': 'unknown',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[BillingEvidenceKeyTypeEnum];
  @override
  final String wireName = 'BillingEvidenceKeyTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    BillingEvidenceKeyTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  BillingEvidenceKeyTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => BillingEvidenceKeyTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$BillingEvidence extends BillingEvidence {
  @override
  final String? provider;
  @override
  final BillingEvidenceKeyTypeEnum keyType;
  @override
  final String? method;

  factory _$BillingEvidence([void Function(BillingEvidenceBuilder)? updates]) =>
      (BillingEvidenceBuilder()..update(updates))._build();

  _$BillingEvidence._({this.provider, required this.keyType, this.method})
    : super._();
  @override
  BillingEvidence rebuild(void Function(BillingEvidenceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  BillingEvidenceBuilder toBuilder() => BillingEvidenceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is BillingEvidence &&
        provider == other.provider &&
        keyType == other.keyType &&
        method == other.method;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, keyType.hashCode);
    _$hash = $jc(_$hash, method.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'BillingEvidence')
          ..add('provider', provider)
          ..add('keyType', keyType)
          ..add('method', method))
        .toString();
  }
}

class BillingEvidenceBuilder
    implements Builder<BillingEvidence, BillingEvidenceBuilder> {
  _$BillingEvidence? _$v;

  String? _provider;
  String? get provider => _$this._provider;
  set provider(String? provider) => _$this._provider = provider;

  BillingEvidenceKeyTypeEnum? _keyType;
  BillingEvidenceKeyTypeEnum? get keyType => _$this._keyType;
  set keyType(BillingEvidenceKeyTypeEnum? keyType) => _$this._keyType = keyType;

  String? _method;
  String? get method => _$this._method;
  set method(String? method) => _$this._method = method;

  BillingEvidenceBuilder() {
    BillingEvidence._defaults(this);
  }

  BillingEvidenceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _provider = $v.provider;
      _keyType = $v.keyType;
      _method = $v.method;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(BillingEvidence other) {
    _$v = other as _$BillingEvidence;
  }

  @override
  void update(void Function(BillingEvidenceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  BillingEvidence build() => _build();

  _$BillingEvidence _build() {
    final _$result =
        _$v ??
        _$BillingEvidence._(
          provider: provider,
          keyType: BuiltValueNullFieldError.checkNotNull(
            keyType,
            r'BillingEvidence',
            'keyType',
          ),
          method: method,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
