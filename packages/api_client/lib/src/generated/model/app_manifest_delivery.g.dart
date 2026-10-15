// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest_delivery.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AppManifestDeliveryModeEnum _$appManifestDeliveryModeEnum_gateway =
    const AppManifestDeliveryModeEnum._('gateway');
const AppManifestDeliveryModeEnum _$appManifestDeliveryModeEnum_webhook =
    const AppManifestDeliveryModeEnum._('webhook');
const AppManifestDeliveryModeEnum
_$appManifestDeliveryModeEnum_unknownDefaultOpenApi =
    const AppManifestDeliveryModeEnum._('unknownDefaultOpenApi');

AppManifestDeliveryModeEnum _$appManifestDeliveryModeEnumValueOf(String name) {
  switch (name) {
    case 'gateway':
      return _$appManifestDeliveryModeEnum_gateway;
    case 'webhook':
      return _$appManifestDeliveryModeEnum_webhook;
    case 'unknownDefaultOpenApi':
      return _$appManifestDeliveryModeEnum_unknownDefaultOpenApi;
    default:
      return _$appManifestDeliveryModeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AppManifestDeliveryModeEnum>
_$appManifestDeliveryModeEnumValues = BuiltSet<AppManifestDeliveryModeEnum>(
  const <AppManifestDeliveryModeEnum>[
    _$appManifestDeliveryModeEnum_gateway,
    _$appManifestDeliveryModeEnum_webhook,
    _$appManifestDeliveryModeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AppManifestDeliveryModeEnum>
_$appManifestDeliveryModeEnumSerializer =
    _$AppManifestDeliveryModeEnumSerializer();

class _$AppManifestDeliveryModeEnumSerializer
    implements PrimitiveSerializer<AppManifestDeliveryModeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'gateway': 'gateway',
    'webhook': 'webhook',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'gateway': 'gateway',
    'webhook': 'webhook',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AppManifestDeliveryModeEnum];
  @override
  final String wireName = 'AppManifestDeliveryModeEnum';

  @override
  Object serialize(
    Serializers serializers,
    AppManifestDeliveryModeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AppManifestDeliveryModeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AppManifestDeliveryModeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AppManifestDelivery extends AppManifestDelivery {
  @override
  final AppManifestDeliveryModeEnum mode;
  @override
  final String? eventsUrl;
  @override
  final String? interactionsUrl;

  factory _$AppManifestDelivery([
    void Function(AppManifestDeliveryBuilder)? updates,
  ]) => (AppManifestDeliveryBuilder()..update(updates))._build();

  _$AppManifestDelivery._({
    required this.mode,
    this.eventsUrl,
    this.interactionsUrl,
  }) : super._();
  @override
  AppManifestDelivery rebuild(
    void Function(AppManifestDeliveryBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AppManifestDeliveryBuilder toBuilder() =>
      AppManifestDeliveryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppManifestDelivery &&
        mode == other.mode &&
        eventsUrl == other.eventsUrl &&
        interactionsUrl == other.interactionsUrl;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, eventsUrl.hashCode);
    _$hash = $jc(_$hash, interactionsUrl.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifestDelivery')
          ..add('mode', mode)
          ..add('eventsUrl', eventsUrl)
          ..add('interactionsUrl', interactionsUrl))
        .toString();
  }
}

class AppManifestDeliveryBuilder
    implements Builder<AppManifestDelivery, AppManifestDeliveryBuilder> {
  _$AppManifestDelivery? _$v;

  AppManifestDeliveryModeEnum? _mode;
  AppManifestDeliveryModeEnum? get mode => _$this._mode;
  set mode(AppManifestDeliveryModeEnum? mode) => _$this._mode = mode;

  String? _eventsUrl;
  String? get eventsUrl => _$this._eventsUrl;
  set eventsUrl(String? eventsUrl) => _$this._eventsUrl = eventsUrl;

  String? _interactionsUrl;
  String? get interactionsUrl => _$this._interactionsUrl;
  set interactionsUrl(String? interactionsUrl) =>
      _$this._interactionsUrl = interactionsUrl;

  AppManifestDeliveryBuilder() {
    AppManifestDelivery._defaults(this);
  }

  AppManifestDeliveryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _mode = $v.mode;
      _eventsUrl = $v.eventsUrl;
      _interactionsUrl = $v.interactionsUrl;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppManifestDelivery other) {
    _$v = other as _$AppManifestDelivery;
  }

  @override
  void update(void Function(AppManifestDeliveryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppManifestDelivery build() => _build();

  _$AppManifestDelivery _build() {
    final _$result =
        _$v ??
        _$AppManifestDelivery._(
          mode: BuiltValueNullFieldError.checkNotNull(
            mode,
            r'AppManifestDelivery',
            'mode',
          ),
          eventsUrl: eventsUrl,
          interactionsUrl: interactionsUrl,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
