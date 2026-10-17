// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarPayload extends SidebarPayload {
  @override
  final SidebarState state;

  factory _$SidebarPayload([void Function(SidebarPayloadBuilder)? updates]) =>
      (SidebarPayloadBuilder()..update(updates))._build();

  _$SidebarPayload._({required this.state}) : super._();
  @override
  SidebarPayload rebuild(void Function(SidebarPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SidebarPayloadBuilder toBuilder() => SidebarPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarPayload && state == other.state;
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
      r'SidebarPayload',
    )..add('state', state)).toString();
  }
}

class SidebarPayloadBuilder
    implements Builder<SidebarPayload, SidebarPayloadBuilder> {
  _$SidebarPayload? _$v;

  SidebarStateBuilder? _state;
  SidebarStateBuilder get state => _$this._state ??= SidebarStateBuilder();
  set state(SidebarStateBuilder? state) => _$this._state = state;

  SidebarPayloadBuilder() {
    SidebarPayload._defaults(this);
  }

  SidebarPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarPayload other) {
    _$v = other as _$SidebarPayload;
  }

  @override
  void update(void Function(SidebarPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarPayload build() => _build();

  _$SidebarPayload _build() {
    _$SidebarPayload _$result;
    try {
      _$result = _$v ?? _$SidebarPayload._(state: state.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'state';
        state.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarPayload',
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
