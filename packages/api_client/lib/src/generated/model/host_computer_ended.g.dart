// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_ended.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostComputerEndedCloseReasonEnum _$hostComputerEndedCloseReasonEnum_done =
    const HostComputerEndedCloseReasonEnum._('done');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_stoppedByUser =
    const HostComputerEndedCloseReasonEnum._('stoppedByUser');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_deviceDisabled =
    const HostComputerEndedCloseReasonEnum._('deviceDisabled');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_connectionLost =
    const HostComputerEndedCloseReasonEnum._('connectionLost');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_openFailed =
    const HostComputerEndedCloseReasonEnum._('openFailed');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_error =
    const HostComputerEndedCloseReasonEnum._('error');
const HostComputerEndedCloseReasonEnum
_$hostComputerEndedCloseReasonEnum_unknownDefaultOpenApi =
    const HostComputerEndedCloseReasonEnum._('unknownDefaultOpenApi');

HostComputerEndedCloseReasonEnum _$hostComputerEndedCloseReasonEnumValueOf(
  String name,
) {
  switch (name) {
    case 'done':
      return _$hostComputerEndedCloseReasonEnum_done;
    case 'stoppedByUser':
      return _$hostComputerEndedCloseReasonEnum_stoppedByUser;
    case 'deviceDisabled':
      return _$hostComputerEndedCloseReasonEnum_deviceDisabled;
    case 'connectionLost':
      return _$hostComputerEndedCloseReasonEnum_connectionLost;
    case 'openFailed':
      return _$hostComputerEndedCloseReasonEnum_openFailed;
    case 'error':
      return _$hostComputerEndedCloseReasonEnum_error;
    case 'unknownDefaultOpenApi':
      return _$hostComputerEndedCloseReasonEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerEndedCloseReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerEndedCloseReasonEnum>
_$hostComputerEndedCloseReasonEnumValues =
    BuiltSet<HostComputerEndedCloseReasonEnum>(
      const <HostComputerEndedCloseReasonEnum>[
        _$hostComputerEndedCloseReasonEnum_done,
        _$hostComputerEndedCloseReasonEnum_stoppedByUser,
        _$hostComputerEndedCloseReasonEnum_deviceDisabled,
        _$hostComputerEndedCloseReasonEnum_connectionLost,
        _$hostComputerEndedCloseReasonEnum_openFailed,
        _$hostComputerEndedCloseReasonEnum_error,
        _$hostComputerEndedCloseReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<HostComputerEndedCloseReasonEnum>
_$hostComputerEndedCloseReasonEnumSerializer =
    _$HostComputerEndedCloseReasonEnumSerializer();

class _$HostComputerEndedCloseReasonEnumSerializer
    implements PrimitiveSerializer<HostComputerEndedCloseReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'done': 'done',
    'stoppedByUser': 'stopped_by_user',
    'deviceDisabled': 'device_disabled',
    'connectionLost': 'connection_lost',
    'openFailed': 'open_failed',
    'error': 'error',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'done': 'done',
    'stopped_by_user': 'stoppedByUser',
    'device_disabled': 'deviceDisabled',
    'connection_lost': 'connectionLost',
    'open_failed': 'openFailed',
    'error': 'error',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerEndedCloseReasonEnum];
  @override
  final String wireName = 'HostComputerEndedCloseReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerEndedCloseReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerEndedCloseReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerEndedCloseReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerEnded extends HostComputerEnded {
  @override
  final String sessionId;
  @override
  final HostComputerEndedCloseReasonEnum closeReason;
  @override
  final ComputerCloseDetail? closeDetail;
  @override
  final BuiltMap<String, int> actionCounts;
  @override
  final BuiltList<HostComputerApp> apps;
  @override
  final int pausedMs;

  factory _$HostComputerEnded([
    void Function(HostComputerEndedBuilder)? updates,
  ]) => (HostComputerEndedBuilder()..update(updates))._build();

  _$HostComputerEnded._({
    required this.sessionId,
    required this.closeReason,
    this.closeDetail,
    required this.actionCounts,
    required this.apps,
    required this.pausedMs,
  }) : super._();
  @override
  HostComputerEnded rebuild(void Function(HostComputerEndedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerEndedBuilder toBuilder() =>
      HostComputerEndedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerEnded &&
        sessionId == other.sessionId &&
        closeReason == other.closeReason &&
        closeDetail == other.closeDetail &&
        actionCounts == other.actionCounts &&
        apps == other.apps &&
        pausedMs == other.pausedMs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, closeReason.hashCode);
    _$hash = $jc(_$hash, closeDetail.hashCode);
    _$hash = $jc(_$hash, actionCounts.hashCode);
    _$hash = $jc(_$hash, apps.hashCode);
    _$hash = $jc(_$hash, pausedMs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerEnded')
          ..add('sessionId', sessionId)
          ..add('closeReason', closeReason)
          ..add('closeDetail', closeDetail)
          ..add('actionCounts', actionCounts)
          ..add('apps', apps)
          ..add('pausedMs', pausedMs))
        .toString();
  }
}

class HostComputerEndedBuilder
    implements Builder<HostComputerEnded, HostComputerEndedBuilder> {
  _$HostComputerEnded? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  HostComputerEndedCloseReasonEnum? _closeReason;
  HostComputerEndedCloseReasonEnum? get closeReason => _$this._closeReason;
  set closeReason(HostComputerEndedCloseReasonEnum? closeReason) =>
      _$this._closeReason = closeReason;

  ComputerCloseDetail? _closeDetail;
  ComputerCloseDetail? get closeDetail => _$this._closeDetail;
  set closeDetail(ComputerCloseDetail? closeDetail) =>
      _$this._closeDetail = closeDetail;

  MapBuilder<String, int>? _actionCounts;
  MapBuilder<String, int> get actionCounts =>
      _$this._actionCounts ??= MapBuilder<String, int>();
  set actionCounts(MapBuilder<String, int>? actionCounts) =>
      _$this._actionCounts = actionCounts;

  ListBuilder<HostComputerApp>? _apps;
  ListBuilder<HostComputerApp> get apps =>
      _$this._apps ??= ListBuilder<HostComputerApp>();
  set apps(ListBuilder<HostComputerApp>? apps) => _$this._apps = apps;

  int? _pausedMs;
  int? get pausedMs => _$this._pausedMs;
  set pausedMs(int? pausedMs) => _$this._pausedMs = pausedMs;

  HostComputerEndedBuilder() {
    HostComputerEnded._defaults(this);
  }

  HostComputerEndedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _closeReason = $v.closeReason;
      _closeDetail = $v.closeDetail;
      _actionCounts = $v.actionCounts.toBuilder();
      _apps = $v.apps.toBuilder();
      _pausedMs = $v.pausedMs;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerEnded other) {
    _$v = other as _$HostComputerEnded;
  }

  @override
  void update(void Function(HostComputerEndedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerEnded build() => _build();

  _$HostComputerEnded _build() {
    _$HostComputerEnded _$result;
    try {
      _$result =
          _$v ??
          _$HostComputerEnded._(
            sessionId: BuiltValueNullFieldError.checkNotNull(
              sessionId,
              r'HostComputerEnded',
              'sessionId',
            ),
            closeReason: BuiltValueNullFieldError.checkNotNull(
              closeReason,
              r'HostComputerEnded',
              'closeReason',
            ),
            closeDetail: closeDetail,
            actionCounts: actionCounts.build(),
            apps: apps.build(),
            pausedMs: BuiltValueNullFieldError.checkNotNull(
              pausedMs,
              r'HostComputerEnded',
              'pausedMs',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'actionCounts';
        actionCounts.build();
        _$failedField = 'apps';
        apps.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostComputerEnded',
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
