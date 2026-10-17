// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_state_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostStateSetStateEnum _$hostStateSetStateEnum_active =
    const HostStateSetStateEnum._('active');
const HostStateSetStateEnum _$hostStateSetStateEnum_disabled =
    const HostStateSetStateEnum._('disabled');
const HostStateSetStateEnum _$hostStateSetStateEnum_retired =
    const HostStateSetStateEnum._('retired');
const HostStateSetStateEnum _$hostStateSetStateEnum_unknownDefaultOpenApi =
    const HostStateSetStateEnum._('unknownDefaultOpenApi');

HostStateSetStateEnum _$hostStateSetStateEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$hostStateSetStateEnum_active;
    case 'disabled':
      return _$hostStateSetStateEnum_disabled;
    case 'retired':
      return _$hostStateSetStateEnum_retired;
    case 'unknownDefaultOpenApi':
      return _$hostStateSetStateEnum_unknownDefaultOpenApi;
    default:
      return _$hostStateSetStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostStateSetStateEnum> _$hostStateSetStateEnumValues =
    BuiltSet<HostStateSetStateEnum>(const <HostStateSetStateEnum>[
      _$hostStateSetStateEnum_active,
      _$hostStateSetStateEnum_disabled,
      _$hostStateSetStateEnum_retired,
      _$hostStateSetStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<HostStateSetStateEnum> _$hostStateSetStateEnumSerializer =
    _$HostStateSetStateEnumSerializer();

class _$HostStateSetStateEnumSerializer
    implements PrimitiveSerializer<HostStateSetStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'disabled': 'disabled',
    'retired': 'retired',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'disabled': 'disabled',
    'retired': 'retired',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostStateSetStateEnum];
  @override
  final String wireName = 'HostStateSetStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostStateSetStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostStateSetStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostStateSetStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostStateSet extends HostStateSet {
  @override
  final HostStateSetStateEnum state;
  @override
  final String reason;

  factory _$HostStateSet([void Function(HostStateSetBuilder)? updates]) =>
      (HostStateSetBuilder()..update(updates))._build();

  _$HostStateSet._({required this.state, required this.reason}) : super._();
  @override
  HostStateSet rebuild(void Function(HostStateSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostStateSetBuilder toBuilder() => HostStateSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostStateSet &&
        state == other.state &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostStateSet')
          ..add('state', state)
          ..add('reason', reason))
        .toString();
  }
}

class HostStateSetBuilder
    implements Builder<HostStateSet, HostStateSetBuilder> {
  _$HostStateSet? _$v;

  HostStateSetStateEnum? _state;
  HostStateSetStateEnum? get state => _$this._state;
  set state(HostStateSetStateEnum? state) => _$this._state = state;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostStateSetBuilder() {
    HostStateSet._defaults(this);
  }

  HostStateSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostStateSet other) {
    _$v = other as _$HostStateSet;
  }

  @override
  void update(void Function(HostStateSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostStateSet build() => _build();

  _$HostStateSet _build() {
    final _$result =
        _$v ??
        _$HostStateSet._(
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'HostStateSet',
            'state',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'HostStateSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
