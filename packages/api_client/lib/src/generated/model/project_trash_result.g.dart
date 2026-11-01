// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_trash_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ProjectTrashResultResultEnum _$projectTrashResultResultEnum_trashed =
    const ProjectTrashResultResultEnum._('trashed');
const ProjectTrashResultResultEnum _$projectTrashResultResultEnum_refused =
    const ProjectTrashResultResultEnum._('refused');
const ProjectTrashResultResultEnum
_$projectTrashResultResultEnum_unknownDefaultOpenApi =
    const ProjectTrashResultResultEnum._('unknownDefaultOpenApi');

ProjectTrashResultResultEnum _$projectTrashResultResultEnumValueOf(
  String name,
) {
  switch (name) {
    case 'trashed':
      return _$projectTrashResultResultEnum_trashed;
    case 'refused':
      return _$projectTrashResultResultEnum_refused;
    case 'unknownDefaultOpenApi':
      return _$projectTrashResultResultEnum_unknownDefaultOpenApi;
    default:
      return _$projectTrashResultResultEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ProjectTrashResultResultEnum>
_$projectTrashResultResultEnumValues = BuiltSet<ProjectTrashResultResultEnum>(
  const <ProjectTrashResultResultEnum>[
    _$projectTrashResultResultEnum_trashed,
    _$projectTrashResultResultEnum_refused,
    _$projectTrashResultResultEnum_unknownDefaultOpenApi,
  ],
);

Serializer<ProjectTrashResultResultEnum>
_$projectTrashResultResultEnumSerializer =
    _$ProjectTrashResultResultEnumSerializer();

class _$ProjectTrashResultResultEnumSerializer
    implements PrimitiveSerializer<ProjectTrashResultResultEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'trashed': 'trashed',
    'refused': 'refused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'trashed': 'trashed',
    'refused': 'refused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ProjectTrashResultResultEnum];
  @override
  final String wireName = 'ProjectTrashResultResultEnum';

  @override
  Object serialize(
    Serializers serializers,
    ProjectTrashResultResultEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ProjectTrashResultResultEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ProjectTrashResultResultEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ProjectTrashResult extends ProjectTrashResult {
  @override
  final String channelId;
  @override
  final ProjectTrashResultResultEnum result;
  @override
  final String? reason;

  factory _$ProjectTrashResult([
    void Function(ProjectTrashResultBuilder)? updates,
  ]) => (ProjectTrashResultBuilder()..update(updates))._build();

  _$ProjectTrashResult._({
    required this.channelId,
    required this.result,
    this.reason,
  }) : super._();
  @override
  ProjectTrashResult rebuild(
    void Function(ProjectTrashResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ProjectTrashResultBuilder toBuilder() =>
      ProjectTrashResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectTrashResult &&
        channelId == other.channelId &&
        result == other.result &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, result.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectTrashResult')
          ..add('channelId', channelId)
          ..add('result', result)
          ..add('reason', reason))
        .toString();
  }
}

class ProjectTrashResultBuilder
    implements Builder<ProjectTrashResult, ProjectTrashResultBuilder> {
  _$ProjectTrashResult? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  ProjectTrashResultResultEnum? _result;
  ProjectTrashResultResultEnum? get result => _$this._result;
  set result(ProjectTrashResultResultEnum? result) => _$this._result = result;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ProjectTrashResultBuilder() {
    ProjectTrashResult._defaults(this);
  }

  ProjectTrashResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _result = $v.result;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectTrashResult other) {
    _$v = other as _$ProjectTrashResult;
  }

  @override
  void update(void Function(ProjectTrashResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectTrashResult build() => _build();

  _$ProjectTrashResult _build() {
    final _$result =
        _$v ??
        _$ProjectTrashResult._(
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ProjectTrashResult',
            'channelId',
          ),
          result: BuiltValueNullFieldError.checkNotNull(
            result,
            r'ProjectTrashResult',
            'result',
          ),
          reason: reason,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
