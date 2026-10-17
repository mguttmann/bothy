// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pairing_status.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PairingStatusStatusEnum _$pairingStatusStatusEnum_requested =
    const PairingStatusStatusEnum._('requested');
const PairingStatusStatusEnum _$pairingStatusStatusEnum_unknownDefaultOpenApi =
    const PairingStatusStatusEnum._('unknownDefaultOpenApi');

PairingStatusStatusEnum _$pairingStatusStatusEnumValueOf(String name) {
  switch (name) {
    case 'requested':
      return _$pairingStatusStatusEnum_requested;
    case 'unknownDefaultOpenApi':
      return _$pairingStatusStatusEnum_unknownDefaultOpenApi;
    default:
      return _$pairingStatusStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<PairingStatusStatusEnum> _$pairingStatusStatusEnumValues =
    BuiltSet<PairingStatusStatusEnum>(const <PairingStatusStatusEnum>[
      _$pairingStatusStatusEnum_requested,
      _$pairingStatusStatusEnum_unknownDefaultOpenApi,
    ]);

Serializer<PairingStatusStatusEnum> _$pairingStatusStatusEnumSerializer =
    _$PairingStatusStatusEnumSerializer();

class _$PairingStatusStatusEnumSerializer
    implements PrimitiveSerializer<PairingStatusStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'requested': 'requested',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'requested': 'requested',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PairingStatusStatusEnum];
  @override
  final String wireName = 'PairingStatusStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    PairingStatusStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PairingStatusStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PairingStatusStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$PairingStatus extends PairingStatus {
  @override
  final String pairingId;
  @override
  final PairingStatusStatusEnum status;
  @override
  final DateTime expiresAt;

  factory _$PairingStatus([void Function(PairingStatusBuilder)? updates]) =>
      (PairingStatusBuilder()..update(updates))._build();

  _$PairingStatus._({
    required this.pairingId,
    required this.status,
    required this.expiresAt,
  }) : super._();
  @override
  PairingStatus rebuild(void Function(PairingStatusBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PairingStatusBuilder toBuilder() => PairingStatusBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PairingStatus &&
        pairingId == other.pairingId &&
        status == other.status &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pairingId.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PairingStatus')
          ..add('pairingId', pairingId)
          ..add('status', status)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class PairingStatusBuilder
    implements Builder<PairingStatus, PairingStatusBuilder> {
  _$PairingStatus? _$v;

  String? _pairingId;
  String? get pairingId => _$this._pairingId;
  set pairingId(String? pairingId) => _$this._pairingId = pairingId;

  PairingStatusStatusEnum? _status;
  PairingStatusStatusEnum? get status => _$this._status;
  set status(PairingStatusStatusEnum? status) => _$this._status = status;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  PairingStatusBuilder() {
    PairingStatus._defaults(this);
  }

  PairingStatusBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pairingId = $v.pairingId;
      _status = $v.status;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PairingStatus other) {
    _$v = other as _$PairingStatus;
  }

  @override
  void update(void Function(PairingStatusBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PairingStatus build() => _build();

  _$PairingStatus _build() {
    final _$result =
        _$v ??
        _$PairingStatus._(
          pairingId: BuiltValueNullFieldError.checkNotNull(
            pairingId,
            r'PairingStatus',
            'pairingId',
          ),
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'PairingStatus',
            'status',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'PairingStatus',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
