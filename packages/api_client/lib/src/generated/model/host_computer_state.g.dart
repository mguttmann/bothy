// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostComputerStateStateEnum _$hostComputerStateStateEnum_active =
    const HostComputerStateStateEnum._('active');
const HostComputerStateStateEnum _$hostComputerStateStateEnum_paused =
    const HostComputerStateStateEnum._('paused');
const HostComputerStateStateEnum
_$hostComputerStateStateEnum_unknownDefaultOpenApi =
    const HostComputerStateStateEnum._('unknownDefaultOpenApi');

HostComputerStateStateEnum _$hostComputerStateStateEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$hostComputerStateStateEnum_active;
    case 'paused':
      return _$hostComputerStateStateEnum_paused;
    case 'unknownDefaultOpenApi':
      return _$hostComputerStateStateEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerStateStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerStateStateEnum> _$hostComputerStateStateEnumValues =
    BuiltSet<HostComputerStateStateEnum>(const <HostComputerStateStateEnum>[
      _$hostComputerStateStateEnum_active,
      _$hostComputerStateStateEnum_paused,
      _$hostComputerStateStateEnum_unknownDefaultOpenApi,
    ]);

const HostComputerStateReasonEnum _$hostComputerStateReasonEnum_empty =
    const HostComputerStateReasonEnum._('empty');
const HostComputerStateReasonEnum _$hostComputerStateReasonEnum_locked =
    const HostComputerStateReasonEnum._('locked');
const HostComputerStateReasonEnum
_$hostComputerStateReasonEnum_unknownDefaultOpenApi =
    const HostComputerStateReasonEnum._('unknownDefaultOpenApi');

HostComputerStateReasonEnum _$hostComputerStateReasonEnumValueOf(String name) {
  switch (name) {
    case 'empty':
      return _$hostComputerStateReasonEnum_empty;
    case 'locked':
      return _$hostComputerStateReasonEnum_locked;
    case 'unknownDefaultOpenApi':
      return _$hostComputerStateReasonEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerStateReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerStateReasonEnum>
_$hostComputerStateReasonEnumValues = BuiltSet<HostComputerStateReasonEnum>(
  const <HostComputerStateReasonEnum>[
    _$hostComputerStateReasonEnum_empty,
    _$hostComputerStateReasonEnum_locked,
    _$hostComputerStateReasonEnum_unknownDefaultOpenApi,
  ],
);

Serializer<HostComputerStateStateEnum> _$hostComputerStateStateEnumSerializer =
    _$HostComputerStateStateEnumSerializer();
Serializer<HostComputerStateReasonEnum>
_$hostComputerStateReasonEnumSerializer =
    _$HostComputerStateReasonEnumSerializer();

class _$HostComputerStateStateEnumSerializer
    implements PrimitiveSerializer<HostComputerStateStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'paused': 'paused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'paused': 'paused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerStateStateEnum];
  @override
  final String wireName = 'HostComputerStateStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerStateStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerStateStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerStateStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerStateReasonEnumSerializer
    implements PrimitiveSerializer<HostComputerStateReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'empty': '',
    'locked': 'locked',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    '': 'empty',
    'locked': 'locked',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerStateReasonEnum];
  @override
  final String wireName = 'HostComputerStateReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerStateReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerStateReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerStateReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerState extends HostComputerState {
  @override
  final String sessionId;
  @override
  final HostComputerStateStateEnum state;
  @override
  final HostComputerStateReasonEnum reason;
  @override
  final int actionCount;
  @override
  final HostComputerApp frontApp;

  factory _$HostComputerState([
    void Function(HostComputerStateBuilder)? updates,
  ]) => (HostComputerStateBuilder()..update(updates))._build();

  _$HostComputerState._({
    required this.sessionId,
    required this.state,
    required this.reason,
    required this.actionCount,
    required this.frontApp,
  }) : super._();
  @override
  HostComputerState rebuild(void Function(HostComputerStateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerStateBuilder toBuilder() =>
      HostComputerStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerState &&
        sessionId == other.sessionId &&
        state == other.state &&
        reason == other.reason &&
        actionCount == other.actionCount &&
        frontApp == other.frontApp;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, actionCount.hashCode);
    _$hash = $jc(_$hash, frontApp.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerState')
          ..add('sessionId', sessionId)
          ..add('state', state)
          ..add('reason', reason)
          ..add('actionCount', actionCount)
          ..add('frontApp', frontApp))
        .toString();
  }
}

class HostComputerStateBuilder
    implements Builder<HostComputerState, HostComputerStateBuilder> {
  _$HostComputerState? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  HostComputerStateStateEnum? _state;
  HostComputerStateStateEnum? get state => _$this._state;
  set state(HostComputerStateStateEnum? state) => _$this._state = state;

  HostComputerStateReasonEnum? _reason;
  HostComputerStateReasonEnum? get reason => _$this._reason;
  set reason(HostComputerStateReasonEnum? reason) => _$this._reason = reason;

  int? _actionCount;
  int? get actionCount => _$this._actionCount;
  set actionCount(int? actionCount) => _$this._actionCount = actionCount;

  HostComputerAppBuilder? _frontApp;
  HostComputerAppBuilder get frontApp =>
      _$this._frontApp ??= HostComputerAppBuilder();
  set frontApp(HostComputerAppBuilder? frontApp) => _$this._frontApp = frontApp;

  HostComputerStateBuilder() {
    HostComputerState._defaults(this);
  }

  HostComputerStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _state = $v.state;
      _reason = $v.reason;
      _actionCount = $v.actionCount;
      _frontApp = $v.frontApp.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerState other) {
    _$v = other as _$HostComputerState;
  }

  @override
  void update(void Function(HostComputerStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerState build() => _build();

  _$HostComputerState _build() {
    _$HostComputerState _$result;
    try {
      _$result =
          _$v ??
          _$HostComputerState._(
            sessionId: BuiltValueNullFieldError.checkNotNull(
              sessionId,
              r'HostComputerState',
              'sessionId',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'HostComputerState',
              'state',
            ),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'HostComputerState',
              'reason',
            ),
            actionCount: BuiltValueNullFieldError.checkNotNull(
              actionCount,
              r'HostComputerState',
              'actionCount',
            ),
            frontApp: frontApp.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'frontApp';
        frontApp.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostComputerState',
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
