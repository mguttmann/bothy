// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'json_patch_operation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const JsonPatchOperationOpEnum _$jsonPatchOperationOpEnum_add =
    const JsonPatchOperationOpEnum._('add');
const JsonPatchOperationOpEnum _$jsonPatchOperationOpEnum_replace =
    const JsonPatchOperationOpEnum._('replace');
const JsonPatchOperationOpEnum _$jsonPatchOperationOpEnum_remove =
    const JsonPatchOperationOpEnum._('remove');
const JsonPatchOperationOpEnum
_$jsonPatchOperationOpEnum_unknownDefaultOpenApi =
    const JsonPatchOperationOpEnum._('unknownDefaultOpenApi');

JsonPatchOperationOpEnum _$jsonPatchOperationOpEnumValueOf(String name) {
  switch (name) {
    case 'add':
      return _$jsonPatchOperationOpEnum_add;
    case 'replace':
      return _$jsonPatchOperationOpEnum_replace;
    case 'remove':
      return _$jsonPatchOperationOpEnum_remove;
    case 'unknownDefaultOpenApi':
      return _$jsonPatchOperationOpEnum_unknownDefaultOpenApi;
    default:
      return _$jsonPatchOperationOpEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<JsonPatchOperationOpEnum> _$jsonPatchOperationOpEnumValues =
    BuiltSet<JsonPatchOperationOpEnum>(const <JsonPatchOperationOpEnum>[
      _$jsonPatchOperationOpEnum_add,
      _$jsonPatchOperationOpEnum_replace,
      _$jsonPatchOperationOpEnum_remove,
      _$jsonPatchOperationOpEnum_unknownDefaultOpenApi,
    ]);

Serializer<JsonPatchOperationOpEnum> _$jsonPatchOperationOpEnumSerializer =
    _$JsonPatchOperationOpEnumSerializer();

class _$JsonPatchOperationOpEnumSerializer
    implements PrimitiveSerializer<JsonPatchOperationOpEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'add': 'add',
    'replace': 'replace',
    'remove': 'remove',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'add': 'add',
    'replace': 'replace',
    'remove': 'remove',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[JsonPatchOperationOpEnum];
  @override
  final String wireName = 'JsonPatchOperationOpEnum';

  @override
  Object serialize(
    Serializers serializers,
    JsonPatchOperationOpEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  JsonPatchOperationOpEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => JsonPatchOperationOpEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$JsonPatchOperation extends JsonPatchOperation {
  @override
  final JsonPatchOperationOpEnum op;
  @override
  final String path;
  @override
  final JsonObject? value;

  factory _$JsonPatchOperation([
    void Function(JsonPatchOperationBuilder)? updates,
  ]) => (JsonPatchOperationBuilder()..update(updates))._build();

  _$JsonPatchOperation._({required this.op, required this.path, this.value})
    : super._();
  @override
  JsonPatchOperation rebuild(
    void Function(JsonPatchOperationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  JsonPatchOperationBuilder toBuilder() =>
      JsonPatchOperationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is JsonPatchOperation &&
        op == other.op &&
        path == other.path &&
        value == other.value;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, op.hashCode);
    _$hash = $jc(_$hash, path.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'JsonPatchOperation')
          ..add('op', op)
          ..add('path', path)
          ..add('value', value))
        .toString();
  }
}

class JsonPatchOperationBuilder
    implements Builder<JsonPatchOperation, JsonPatchOperationBuilder> {
  _$JsonPatchOperation? _$v;

  JsonPatchOperationOpEnum? _op;
  JsonPatchOperationOpEnum? get op => _$this._op;
  set op(JsonPatchOperationOpEnum? op) => _$this._op = op;

  String? _path;
  String? get path => _$this._path;
  set path(String? path) => _$this._path = path;

  JsonObject? _value;
  JsonObject? get value => _$this._value;
  set value(JsonObject? value) => _$this._value = value;

  JsonPatchOperationBuilder() {
    JsonPatchOperation._defaults(this);
  }

  JsonPatchOperationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _op = $v.op;
      _path = $v.path;
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(JsonPatchOperation other) {
    _$v = other as _$JsonPatchOperation;
  }

  @override
  void update(void Function(JsonPatchOperationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  JsonPatchOperation build() => _build();

  _$JsonPatchOperation _build() {
    final _$result =
        _$v ??
        _$JsonPatchOperation._(
          op: BuiltValueNullFieldError.checkNotNull(
            op,
            r'JsonPatchOperation',
            'op',
          ),
          path: BuiltValueNullFieldError.checkNotNull(
            path,
            r'JsonPatchOperation',
            'path',
          ),
          value: value,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
