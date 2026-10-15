// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_installation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AppInstallationStateEnum _$appInstallationStateEnum_active =
    const AppInstallationStateEnum._('active');
const AppInstallationStateEnum _$appInstallationStateEnum_suspended =
    const AppInstallationStateEnum._('suspended');
const AppInstallationStateEnum _$appInstallationStateEnum_uninstalled =
    const AppInstallationStateEnum._('uninstalled');
const AppInstallationStateEnum
_$appInstallationStateEnum_unknownDefaultOpenApi =
    const AppInstallationStateEnum._('unknownDefaultOpenApi');

AppInstallationStateEnum _$appInstallationStateEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$appInstallationStateEnum_active;
    case 'suspended':
      return _$appInstallationStateEnum_suspended;
    case 'uninstalled':
      return _$appInstallationStateEnum_uninstalled;
    case 'unknownDefaultOpenApi':
      return _$appInstallationStateEnum_unknownDefaultOpenApi;
    default:
      return _$appInstallationStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AppInstallationStateEnum> _$appInstallationStateEnumValues =
    BuiltSet<AppInstallationStateEnum>(const <AppInstallationStateEnum>[
      _$appInstallationStateEnum_active,
      _$appInstallationStateEnum_suspended,
      _$appInstallationStateEnum_uninstalled,
      _$appInstallationStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<AppInstallationStateEnum> _$appInstallationStateEnumSerializer =
    _$AppInstallationStateEnumSerializer();

class _$AppInstallationStateEnumSerializer
    implements PrimitiveSerializer<AppInstallationStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'suspended': 'suspended',
    'uninstalled': 'uninstalled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'suspended': 'suspended',
    'uninstalled': 'uninstalled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AppInstallationStateEnum];
  @override
  final String wireName = 'AppInstallationStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    AppInstallationStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AppInstallationStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AppInstallationStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AppInstallation extends AppInstallation {
  @override
  final String id;
  @override
  final BuiltList<AppScope> grantedScopes;
  @override
  final BuiltList<String> eventSubscriptions;
  @override
  final AppInstallationStateEnum state;
  @override
  final String? installedBy;
  @override
  final DateTime installedAt;

  factory _$AppInstallation([void Function(AppInstallationBuilder)? updates]) =>
      (AppInstallationBuilder()..update(updates))._build();

  _$AppInstallation._({
    required this.id,
    required this.grantedScopes,
    required this.eventSubscriptions,
    required this.state,
    this.installedBy,
    required this.installedAt,
  }) : super._();
  @override
  AppInstallation rebuild(void Function(AppInstallationBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppInstallationBuilder toBuilder() => AppInstallationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppInstallation &&
        id == other.id &&
        grantedScopes == other.grantedScopes &&
        eventSubscriptions == other.eventSubscriptions &&
        state == other.state &&
        installedBy == other.installedBy &&
        installedAt == other.installedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, grantedScopes.hashCode);
    _$hash = $jc(_$hash, eventSubscriptions.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, installedBy.hashCode);
    _$hash = $jc(_$hash, installedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppInstallation')
          ..add('id', id)
          ..add('grantedScopes', grantedScopes)
          ..add('eventSubscriptions', eventSubscriptions)
          ..add('state', state)
          ..add('installedBy', installedBy)
          ..add('installedAt', installedAt))
        .toString();
  }
}

class AppInstallationBuilder
    implements Builder<AppInstallation, AppInstallationBuilder> {
  _$AppInstallation? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

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

  AppInstallationStateEnum? _state;
  AppInstallationStateEnum? get state => _$this._state;
  set state(AppInstallationStateEnum? state) => _$this._state = state;

  String? _installedBy;
  String? get installedBy => _$this._installedBy;
  set installedBy(String? installedBy) => _$this._installedBy = installedBy;

  DateTime? _installedAt;
  DateTime? get installedAt => _$this._installedAt;
  set installedAt(DateTime? installedAt) => _$this._installedAt = installedAt;

  AppInstallationBuilder() {
    AppInstallation._defaults(this);
  }

  AppInstallationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _grantedScopes = $v.grantedScopes.toBuilder();
      _eventSubscriptions = $v.eventSubscriptions.toBuilder();
      _state = $v.state;
      _installedBy = $v.installedBy;
      _installedAt = $v.installedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppInstallation other) {
    _$v = other as _$AppInstallation;
  }

  @override
  void update(void Function(AppInstallationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppInstallation build() => _build();

  _$AppInstallation _build() {
    _$AppInstallation _$result;
    try {
      _$result =
          _$v ??
          _$AppInstallation._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'AppInstallation',
              'id',
            ),
            grantedScopes: grantedScopes.build(),
            eventSubscriptions: eventSubscriptions.build(),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'AppInstallation',
              'state',
            ),
            installedBy: installedBy,
            installedAt: BuiltValueNullFieldError.checkNotNull(
              installedAt,
              r'AppInstallation',
              'installedAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'grantedScopes';
        grantedScopes.build();
        _$failedField = 'eventSubscriptions';
        eventSubscriptions.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppInstallation',
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
