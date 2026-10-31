// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_channel_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarChannelPayload extends SidebarChannelPayload {
  @override
  final SidebarChannelState state;

  factory _$SidebarChannelPayload([
    void Function(SidebarChannelPayloadBuilder)? updates,
  ]) => (SidebarChannelPayloadBuilder()..update(updates))._build();

  _$SidebarChannelPayload._({required this.state}) : super._();
  @override
  SidebarChannelPayload rebuild(
    void Function(SidebarChannelPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SidebarChannelPayloadBuilder toBuilder() =>
      SidebarChannelPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarChannelPayload && state == other.state;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'SidebarChannelPayload',
    )..add('state', state)).toString();
  }
}

class SidebarChannelPayloadBuilder
    implements Builder<SidebarChannelPayload, SidebarChannelPayloadBuilder> {
  _$SidebarChannelPayload? _$v;

  SidebarChannelStateBuilder? _state;
  SidebarChannelStateBuilder get state =>
      _$this._state ??= SidebarChannelStateBuilder();
  set state(SidebarChannelStateBuilder? state) => _$this._state = state;

  SidebarChannelPayloadBuilder() {
    SidebarChannelPayload._defaults(this);
  }

  SidebarChannelPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarChannelPayload other) {
    _$v = other as _$SidebarChannelPayload;
  }

  @override
  void update(void Function(SidebarChannelPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarChannelPayload build() => _build();

  _$SidebarChannelPayload _build() {
    _$SidebarChannelPayload _$result;
    try {
      _$result = _$v ?? _$SidebarChannelPayload._(state: state.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'state';
        state.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarChannelPayload',
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
