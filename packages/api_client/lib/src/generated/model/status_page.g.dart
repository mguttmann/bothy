// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusPage extends StatusPage {
  @override
  final StatusComponentState overallState;
  @override
  final BuiltList<StatusComponent> components;
  @override
  final BuiltList<StatusIncident> incidents;

  factory _$StatusPage([void Function(StatusPageBuilder)? updates]) =>
      (StatusPageBuilder()..update(updates))._build();

  _$StatusPage._({
    required this.overallState,
    required this.components,
    required this.incidents,
  }) : super._();
  @override
  StatusPage rebuild(void Function(StatusPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatusPageBuilder toBuilder() => StatusPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusPage &&
        overallState == other.overallState &&
        components == other.components &&
        incidents == other.incidents;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, overallState.hashCode);
    _$hash = $jc(_$hash, components.hashCode);
    _$hash = $jc(_$hash, incidents.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusPage')
          ..add('overallState', overallState)
          ..add('components', components)
          ..add('incidents', incidents))
        .toString();
  }
}

class StatusPageBuilder implements Builder<StatusPage, StatusPageBuilder> {
  _$StatusPage? _$v;

  StatusComponentState? _overallState;
  StatusComponentState? get overallState => _$this._overallState;
  set overallState(StatusComponentState? overallState) =>
      _$this._overallState = overallState;

  ListBuilder<StatusComponent>? _components;
  ListBuilder<StatusComponent> get components =>
      _$this._components ??= ListBuilder<StatusComponent>();
  set components(ListBuilder<StatusComponent>? components) =>
      _$this._components = components;

  ListBuilder<StatusIncident>? _incidents;
  ListBuilder<StatusIncident> get incidents =>
      _$this._incidents ??= ListBuilder<StatusIncident>();
  set incidents(ListBuilder<StatusIncident>? incidents) =>
      _$this._incidents = incidents;

  StatusPageBuilder() {
    StatusPage._defaults(this);
  }

  StatusPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _overallState = $v.overallState;
      _components = $v.components.toBuilder();
      _incidents = $v.incidents.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusPage other) {
    _$v = other as _$StatusPage;
  }

  @override
  void update(void Function(StatusPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusPage build() => _build();

  _$StatusPage _build() {
    _$StatusPage _$result;
    try {
      _$result =
          _$v ??
          _$StatusPage._(
            overallState: BuiltValueNullFieldError.checkNotNull(
              overallState,
              r'StatusPage',
              'overallState',
            ),
            components: components.build(),
            incidents: incidents.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'components';
        components.build();
        _$failedField = 'incidents';
        incidents.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatusPage',
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
