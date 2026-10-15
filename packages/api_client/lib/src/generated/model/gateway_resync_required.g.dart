// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_resync_required.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GatewayResyncRequiredReasonEnum
_$gatewayResyncRequiredReasonEnum_epochChanged =
    const GatewayResyncRequiredReasonEnum._('epochChanged');
const GatewayResyncRequiredReasonEnum
_$gatewayResyncRequiredReasonEnum_retention =
    const GatewayResyncRequiredReasonEnum._('retention');
const GatewayResyncRequiredReasonEnum
_$gatewayResyncRequiredReasonEnum_tooFarBehind =
    const GatewayResyncRequiredReasonEnum._('tooFarBehind');
const GatewayResyncRequiredReasonEnum
_$gatewayResyncRequiredReasonEnum_cursorAhead =
    const GatewayResyncRequiredReasonEnum._('cursorAhead');
const GatewayResyncRequiredReasonEnum
_$gatewayResyncRequiredReasonEnum_unknownDefaultOpenApi =
    const GatewayResyncRequiredReasonEnum._('unknownDefaultOpenApi');

GatewayResyncRequiredReasonEnum _$gatewayResyncRequiredReasonEnumValueOf(
  String name,
) {
  switch (name) {
    case 'epochChanged':
      return _$gatewayResyncRequiredReasonEnum_epochChanged;
    case 'retention':
      return _$gatewayResyncRequiredReasonEnum_retention;
    case 'tooFarBehind':
      return _$gatewayResyncRequiredReasonEnum_tooFarBehind;
    case 'cursorAhead':
      return _$gatewayResyncRequiredReasonEnum_cursorAhead;
    case 'unknownDefaultOpenApi':
      return _$gatewayResyncRequiredReasonEnum_unknownDefaultOpenApi;
    default:
      return _$gatewayResyncRequiredReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<GatewayResyncRequiredReasonEnum>
_$gatewayResyncRequiredReasonEnumValues =
    BuiltSet<GatewayResyncRequiredReasonEnum>(
      const <GatewayResyncRequiredReasonEnum>[
        _$gatewayResyncRequiredReasonEnum_epochChanged,
        _$gatewayResyncRequiredReasonEnum_retention,
        _$gatewayResyncRequiredReasonEnum_tooFarBehind,
        _$gatewayResyncRequiredReasonEnum_cursorAhead,
        _$gatewayResyncRequiredReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<GatewayResyncRequiredReasonEnum>
_$gatewayResyncRequiredReasonEnumSerializer =
    _$GatewayResyncRequiredReasonEnumSerializer();

class _$GatewayResyncRequiredReasonEnumSerializer
    implements PrimitiveSerializer<GatewayResyncRequiredReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'epochChanged': 'epoch_changed',
    'retention': 'retention',
    'tooFarBehind': 'too_far_behind',
    'cursorAhead': 'cursor_ahead',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'epoch_changed': 'epochChanged',
    'retention': 'retention',
    'too_far_behind': 'tooFarBehind',
    'cursor_ahead': 'cursorAhead',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GatewayResyncRequiredReasonEnum];
  @override
  final String wireName = 'GatewayResyncRequiredReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    GatewayResyncRequiredReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GatewayResyncRequiredReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GatewayResyncRequiredReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$GatewayResyncRequired extends GatewayResyncRequired {
  @override
  final GatewayResyncRequiredReasonEnum reason;
  @override
  final int headSeq;
  @override
  final String syncEpoch;

  factory _$GatewayResyncRequired([
    void Function(GatewayResyncRequiredBuilder)? updates,
  ]) => (GatewayResyncRequiredBuilder()..update(updates))._build();

  _$GatewayResyncRequired._({
    required this.reason,
    required this.headSeq,
    required this.syncEpoch,
  }) : super._();
  @override
  GatewayResyncRequired rebuild(
    void Function(GatewayResyncRequiredBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayResyncRequiredBuilder toBuilder() =>
      GatewayResyncRequiredBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayResyncRequired &&
        reason == other.reason &&
        headSeq == other.headSeq &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayResyncRequired')
          ..add('reason', reason)
          ..add('headSeq', headSeq)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class GatewayResyncRequiredBuilder
    implements Builder<GatewayResyncRequired, GatewayResyncRequiredBuilder> {
  _$GatewayResyncRequired? _$v;

  GatewayResyncRequiredReasonEnum? _reason;
  GatewayResyncRequiredReasonEnum? get reason => _$this._reason;
  set reason(GatewayResyncRequiredReasonEnum? reason) =>
      _$this._reason = reason;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayResyncRequiredBuilder() {
    GatewayResyncRequired._defaults(this);
  }

  GatewayResyncRequiredBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reason = $v.reason;
      _headSeq = $v.headSeq;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayResyncRequired other) {
    _$v = other as _$GatewayResyncRequired;
  }

  @override
  void update(void Function(GatewayResyncRequiredBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayResyncRequired build() => _build();

  _$GatewayResyncRequired _build() {
    final _$result =
        _$v ??
        _$GatewayResyncRequired._(
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'GatewayResyncRequired',
            'reason',
          ),
          headSeq: BuiltValueNullFieldError.checkNotNull(
            headSeq,
            r'GatewayResyncRequired',
            'headSeq',
          ),
          syncEpoch: BuiltValueNullFieldError.checkNotNull(
            syncEpoch,
            r'GatewayResyncRequired',
            'syncEpoch',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
