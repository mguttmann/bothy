// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GrantChangedPayloadChangeEnum _$grantChangedPayloadChangeEnum_created =
    const GrantChangedPayloadChangeEnum._('created');
const GrantChangedPayloadChangeEnum _$grantChangedPayloadChangeEnum_revoked =
    const GrantChangedPayloadChangeEnum._('revoked');
const GrantChangedPayloadChangeEnum _$grantChangedPayloadChangeEnum_expired =
    const GrantChangedPayloadChangeEnum._('expired');
const GrantChangedPayloadChangeEnum
_$grantChangedPayloadChangeEnum_unknownDefaultOpenApi =
    const GrantChangedPayloadChangeEnum._('unknownDefaultOpenApi');

GrantChangedPayloadChangeEnum _$grantChangedPayloadChangeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'created':
      return _$grantChangedPayloadChangeEnum_created;
    case 'revoked':
      return _$grantChangedPayloadChangeEnum_revoked;
    case 'expired':
      return _$grantChangedPayloadChangeEnum_expired;
    case 'unknownDefaultOpenApi':
      return _$grantChangedPayloadChangeEnum_unknownDefaultOpenApi;
    default:
      return _$grantChangedPayloadChangeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<GrantChangedPayloadChangeEnum>
_$grantChangedPayloadChangeEnumValues = BuiltSet<GrantChangedPayloadChangeEnum>(
  const <GrantChangedPayloadChangeEnum>[
    _$grantChangedPayloadChangeEnum_created,
    _$grantChangedPayloadChangeEnum_revoked,
    _$grantChangedPayloadChangeEnum_expired,
    _$grantChangedPayloadChangeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<GrantChangedPayloadChangeEnum>
_$grantChangedPayloadChangeEnumSerializer =
    _$GrantChangedPayloadChangeEnumSerializer();

class _$GrantChangedPayloadChangeEnumSerializer
    implements PrimitiveSerializer<GrantChangedPayloadChangeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'created': 'created',
    'revoked': 'revoked',
    'expired': 'expired',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'created': 'created',
    'revoked': 'revoked',
    'expired': 'expired',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GrantChangedPayloadChangeEnum];
  @override
  final String wireName = 'GrantChangedPayloadChangeEnum';

  @override
  Object serialize(
    Serializers serializers,
    GrantChangedPayloadChangeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GrantChangedPayloadChangeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GrantChangedPayloadChangeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$GrantChangedPayload extends GrantChangedPayload {
  @override
  final Grant grant;
  @override
  final GrantChangedPayloadChangeEnum change;

  factory _$GrantChangedPayload([
    void Function(GrantChangedPayloadBuilder)? updates,
  ]) => (GrantChangedPayloadBuilder()..update(updates))._build();

  _$GrantChangedPayload._({required this.grant, required this.change})
    : super._();
  @override
  GrantChangedPayload rebuild(
    void Function(GrantChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GrantChangedPayloadBuilder toBuilder() =>
      GrantChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GrantChangedPayload &&
        grant == other.grant &&
        change == other.change;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, grant.hashCode);
    _$hash = $jc(_$hash, change.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GrantChangedPayload')
          ..add('grant', grant)
          ..add('change', change))
        .toString();
  }
}

class GrantChangedPayloadBuilder
    implements Builder<GrantChangedPayload, GrantChangedPayloadBuilder> {
  _$GrantChangedPayload? _$v;

  GrantBuilder? _grant;
  GrantBuilder get grant => _$this._grant ??= GrantBuilder();
  set grant(GrantBuilder? grant) => _$this._grant = grant;

  GrantChangedPayloadChangeEnum? _change;
  GrantChangedPayloadChangeEnum? get change => _$this._change;
  set change(GrantChangedPayloadChangeEnum? change) => _$this._change = change;

  GrantChangedPayloadBuilder() {
    GrantChangedPayload._defaults(this);
  }

  GrantChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _grant = $v.grant.toBuilder();
      _change = $v.change;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GrantChangedPayload other) {
    _$v = other as _$GrantChangedPayload;
  }

  @override
  void update(void Function(GrantChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GrantChangedPayload build() => _build();

  _$GrantChangedPayload _build() {
    _$GrantChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$GrantChangedPayload._(
            grant: grant.build(),
            change: BuiltValueNullFieldError.checkNotNull(
              change,
              r'GrantChangedPayload',
              'change',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'grant';
        grant.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GrantChangedPayload',
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
