// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'audit_verify_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_hashMismatch =
    const AuditVerifyResultFailureReasonEnum._('hashMismatch');
const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_linkMismatch =
    const AuditVerifyResultFailureReasonEnum._('linkMismatch');
const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_seqGap =
    const AuditVerifyResultFailureReasonEnum._('seqGap');
const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_checkpointMismatch =
    const AuditVerifyResultFailureReasonEnum._('checkpointMismatch');
const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_checkpointInvalid =
    const AuditVerifyResultFailureReasonEnum._('checkpointInvalid');
const AuditVerifyResultFailureReasonEnum
_$auditVerifyResultFailureReasonEnum_unknownDefaultOpenApi =
    const AuditVerifyResultFailureReasonEnum._('unknownDefaultOpenApi');

AuditVerifyResultFailureReasonEnum _$auditVerifyResultFailureReasonEnumValueOf(
  String name,
) {
  switch (name) {
    case 'hashMismatch':
      return _$auditVerifyResultFailureReasonEnum_hashMismatch;
    case 'linkMismatch':
      return _$auditVerifyResultFailureReasonEnum_linkMismatch;
    case 'seqGap':
      return _$auditVerifyResultFailureReasonEnum_seqGap;
    case 'checkpointMismatch':
      return _$auditVerifyResultFailureReasonEnum_checkpointMismatch;
    case 'checkpointInvalid':
      return _$auditVerifyResultFailureReasonEnum_checkpointInvalid;
    case 'unknownDefaultOpenApi':
      return _$auditVerifyResultFailureReasonEnum_unknownDefaultOpenApi;
    default:
      return _$auditVerifyResultFailureReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AuditVerifyResultFailureReasonEnum>
_$auditVerifyResultFailureReasonEnumValues =
    BuiltSet<AuditVerifyResultFailureReasonEnum>(
      const <AuditVerifyResultFailureReasonEnum>[
        _$auditVerifyResultFailureReasonEnum_hashMismatch,
        _$auditVerifyResultFailureReasonEnum_linkMismatch,
        _$auditVerifyResultFailureReasonEnum_seqGap,
        _$auditVerifyResultFailureReasonEnum_checkpointMismatch,
        _$auditVerifyResultFailureReasonEnum_checkpointInvalid,
        _$auditVerifyResultFailureReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AuditVerifyResultFailureReasonEnum>
_$auditVerifyResultFailureReasonEnumSerializer =
    _$AuditVerifyResultFailureReasonEnumSerializer();

class _$AuditVerifyResultFailureReasonEnumSerializer
    implements PrimitiveSerializer<AuditVerifyResultFailureReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'hashMismatch': 'hash_mismatch',
    'linkMismatch': 'link_mismatch',
    'seqGap': 'seq_gap',
    'checkpointMismatch': 'checkpoint_mismatch',
    'checkpointInvalid': 'checkpoint_invalid',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'hash_mismatch': 'hashMismatch',
    'link_mismatch': 'linkMismatch',
    'seq_gap': 'seqGap',
    'checkpoint_mismatch': 'checkpointMismatch',
    'checkpoint_invalid': 'checkpointInvalid',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AuditVerifyResultFailureReasonEnum];
  @override
  final String wireName = 'AuditVerifyResultFailureReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    AuditVerifyResultFailureReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AuditVerifyResultFailureReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AuditVerifyResultFailureReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AuditVerifyResult extends AuditVerifyResult {
  @override
  final bool ok;
  @override
  final int checkedFromSeq;
  @override
  final int checkedToSeq;
  @override
  final int? firstBrokenSeq;
  @override
  final AuditVerifyResultFailureReasonEnum? failureReason;
  @override
  final int checkpointsCompared;
  @override
  final int? lastCheckpointSeq;
  @override
  final BuiltList<String> warnings;

  factory _$AuditVerifyResult([
    void Function(AuditVerifyResultBuilder)? updates,
  ]) => (AuditVerifyResultBuilder()..update(updates))._build();

  _$AuditVerifyResult._({
    required this.ok,
    required this.checkedFromSeq,
    required this.checkedToSeq,
    this.firstBrokenSeq,
    this.failureReason,
    required this.checkpointsCompared,
    this.lastCheckpointSeq,
    required this.warnings,
  }) : super._();
  @override
  AuditVerifyResult rebuild(void Function(AuditVerifyResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AuditVerifyResultBuilder toBuilder() =>
      AuditVerifyResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AuditVerifyResult &&
        ok == other.ok &&
        checkedFromSeq == other.checkedFromSeq &&
        checkedToSeq == other.checkedToSeq &&
        firstBrokenSeq == other.firstBrokenSeq &&
        failureReason == other.failureReason &&
        checkpointsCompared == other.checkpointsCompared &&
        lastCheckpointSeq == other.lastCheckpointSeq &&
        warnings == other.warnings;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, ok.hashCode);
    _$hash = $jc(_$hash, checkedFromSeq.hashCode);
    _$hash = $jc(_$hash, checkedToSeq.hashCode);
    _$hash = $jc(_$hash, firstBrokenSeq.hashCode);
    _$hash = $jc(_$hash, failureReason.hashCode);
    _$hash = $jc(_$hash, checkpointsCompared.hashCode);
    _$hash = $jc(_$hash, lastCheckpointSeq.hashCode);
    _$hash = $jc(_$hash, warnings.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AuditVerifyResult')
          ..add('ok', ok)
          ..add('checkedFromSeq', checkedFromSeq)
          ..add('checkedToSeq', checkedToSeq)
          ..add('firstBrokenSeq', firstBrokenSeq)
          ..add('failureReason', failureReason)
          ..add('checkpointsCompared', checkpointsCompared)
          ..add('lastCheckpointSeq', lastCheckpointSeq)
          ..add('warnings', warnings))
        .toString();
  }
}

class AuditVerifyResultBuilder
    implements Builder<AuditVerifyResult, AuditVerifyResultBuilder> {
  _$AuditVerifyResult? _$v;

  bool? _ok;
  bool? get ok => _$this._ok;
  set ok(bool? ok) => _$this._ok = ok;

  int? _checkedFromSeq;
  int? get checkedFromSeq => _$this._checkedFromSeq;
  set checkedFromSeq(int? checkedFromSeq) =>
      _$this._checkedFromSeq = checkedFromSeq;

  int? _checkedToSeq;
  int? get checkedToSeq => _$this._checkedToSeq;
  set checkedToSeq(int? checkedToSeq) => _$this._checkedToSeq = checkedToSeq;

  int? _firstBrokenSeq;
  int? get firstBrokenSeq => _$this._firstBrokenSeq;
  set firstBrokenSeq(int? firstBrokenSeq) =>
      _$this._firstBrokenSeq = firstBrokenSeq;

  AuditVerifyResultFailureReasonEnum? _failureReason;
  AuditVerifyResultFailureReasonEnum? get failureReason =>
      _$this._failureReason;
  set failureReason(AuditVerifyResultFailureReasonEnum? failureReason) =>
      _$this._failureReason = failureReason;

  int? _checkpointsCompared;
  int? get checkpointsCompared => _$this._checkpointsCompared;
  set checkpointsCompared(int? checkpointsCompared) =>
      _$this._checkpointsCompared = checkpointsCompared;

  int? _lastCheckpointSeq;
  int? get lastCheckpointSeq => _$this._lastCheckpointSeq;
  set lastCheckpointSeq(int? lastCheckpointSeq) =>
      _$this._lastCheckpointSeq = lastCheckpointSeq;

  ListBuilder<String>? _warnings;
  ListBuilder<String> get warnings =>
      _$this._warnings ??= ListBuilder<String>();
  set warnings(ListBuilder<String>? warnings) => _$this._warnings = warnings;

  AuditVerifyResultBuilder() {
    AuditVerifyResult._defaults(this);
  }

  AuditVerifyResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _ok = $v.ok;
      _checkedFromSeq = $v.checkedFromSeq;
      _checkedToSeq = $v.checkedToSeq;
      _firstBrokenSeq = $v.firstBrokenSeq;
      _failureReason = $v.failureReason;
      _checkpointsCompared = $v.checkpointsCompared;
      _lastCheckpointSeq = $v.lastCheckpointSeq;
      _warnings = $v.warnings.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AuditVerifyResult other) {
    _$v = other as _$AuditVerifyResult;
  }

  @override
  void update(void Function(AuditVerifyResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AuditVerifyResult build() => _build();

  _$AuditVerifyResult _build() {
    _$AuditVerifyResult _$result;
    try {
      _$result =
          _$v ??
          _$AuditVerifyResult._(
            ok: BuiltValueNullFieldError.checkNotNull(
              ok,
              r'AuditVerifyResult',
              'ok',
            ),
            checkedFromSeq: BuiltValueNullFieldError.checkNotNull(
              checkedFromSeq,
              r'AuditVerifyResult',
              'checkedFromSeq',
            ),
            checkedToSeq: BuiltValueNullFieldError.checkNotNull(
              checkedToSeq,
              r'AuditVerifyResult',
              'checkedToSeq',
            ),
            firstBrokenSeq: firstBrokenSeq,
            failureReason: failureReason,
            checkpointsCompared: BuiltValueNullFieldError.checkNotNull(
              checkpointsCompared,
              r'AuditVerifyResult',
              'checkpointsCompared',
            ),
            lastCheckpointSeq: lastCheckpointSeq,
            warnings: warnings.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'warnings';
        warnings.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AuditVerifyResult',
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
