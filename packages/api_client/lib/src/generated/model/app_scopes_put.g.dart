// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_scopes_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppScopesPut extends AppScopesPut {
  @override
  final BuiltList<AppScope> grantedScopes;
  @override
  final BuiltList<String>? eventSubscriptions;
  @override
  final String reason;

  factory _$AppScopesPut([void Function(AppScopesPutBuilder)? updates]) =>
      (AppScopesPutBuilder()..update(updates))._build();

  _$AppScopesPut._({
    required this.grantedScopes,
    this.eventSubscriptions,
    required this.reason,
  }) : super._();
  @override
  AppScopesPut rebuild(void Function(AppScopesPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppScopesPutBuilder toBuilder() => AppScopesPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppScopesPut &&
        grantedScopes == other.grantedScopes &&
        eventSubscriptions == other.eventSubscriptions &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, grantedScopes.hashCode);
    _$hash = $jc(_$hash, eventSubscriptions.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppScopesPut')
          ..add('grantedScopes', grantedScopes)
          ..add('eventSubscriptions', eventSubscriptions)
          ..add('reason', reason))
        .toString();
  }
}

class AppScopesPutBuilder
    implements Builder<AppScopesPut, AppScopesPutBuilder> {
  _$AppScopesPut? _$v;

  ListBuilder<AppScope>? _grantedScopes;
  ListBuilder<AppScope> get grantedScopes =>
      _$this._grantedScopes ??= ListBuilder<AppScope>();
  set grantedScopes(ListBuilder<AppScope>? grantedScopes) =>
      _$this._grantedScopes = grantedScopes;

  ListBuilder<String>? _eventSubscriptions;
  ListBuilder<String> get eventSubscriptions =>
      _$this._eventSubscriptions ??= ListBuilder<String>();
  set eventSubscriptions(ListBuilder<String>? eventSubscriptions) =>
      _$this._eventSubscriptions = eventSubscriptions;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  AppScopesPutBuilder() {
    AppScopesPut._defaults(this);
  }

  AppScopesPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _grantedScopes = $v.grantedScopes.toBuilder();
      _eventSubscriptions = $v.eventSubscriptions?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppScopesPut other) {
    _$v = other as _$AppScopesPut;
  }

  @override
  void update(void Function(AppScopesPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppScopesPut build() => _build();

  _$AppScopesPut _build() {
    _$AppScopesPut _$result;
    try {
      _$result =
          _$v ??
          _$AppScopesPut._(
            grantedScopes: grantedScopes.build(),
            eventSubscriptions: _eventSubscriptions?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'AppScopesPut',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'grantedScopes';
        grantedScopes.build();
        _$failedField = 'eventSubscriptions';
        _eventSubscriptions?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppScopesPut',
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
