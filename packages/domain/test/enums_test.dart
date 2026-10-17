// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

import 'support.dart';

/// Each enum of the domain with the schema of `docs/api/openapi.yaml` it mirrors: a top-level
/// schema name or `Schema.property`.
final Map<String, List<Coded>> _enums = {
  'ChannelKind': ChannelKind.values,
  'ForumSort': ForumSort.values,
  'OperatorMode': OperatorMode.values,
  'NotificationLevel': NotificationLevel.values,
  'NotificationPreference.target_kind': NotificationTargetKind.values,
  'Thread.kind': ThreadKind.values,
  'ThreadState': ThreadState.values,
  'Thread.title_source': TitleSource.values,
  'MessageKind': MessageKind.values,
  'MessageOrigin': MessageOrigin.values,
  'TriggerState': TriggerState.values,
  'Message.system_code': SystemCode.values,
  'File.purpose': FilePurpose.values,
  'File.upload_state': UploadState.values,
  'File.scan_status': ScanStatus.values,
  'TranscriptStatus': TranscriptStatus.values,
  'TranscriptSource': TranscriptSource.values,
  'Thumbnail.variant': ThumbnailVariant.values,
  'ComponentType': ComponentType.values,
  'ComponentRestriction': ComponentRestriction.values,
  'Component.state': ComponentState.values,
  'Capability': Capability.values,
  'RunState': RunState.values,
  'Run.trigger_kind': RunTriggerKind.values,
  'RunStep.kind': StepKind.values,
  'RunStep.status': StepStatus.values,
  'PrincipalKind': PrincipalKind.values,
  'UserStatus': PrincipalStatus.values,
  'BillingMode': BillingMode.values,
  'ToolClass': ToolClass.values,
  'AgentRuntimeState': AgentRuntimeState.values,
  'TargetKind': TargetKind.values,
  'CommandLevel': CommandLevel.values,
  'CommandOption.type': CommandOptionType.values,
};

/// Decoders by schema, to check that every code round-trips and `rocket` is unknown.
final Map<String, Coded Function(String)> _decoders = {
  'ChannelKind': ChannelKind.fromCode,
  'ForumSort': ForumSort.fromCode,
  'OperatorMode': OperatorMode.fromCode,
  'NotificationLevel': NotificationLevel.fromCode,
  'NotificationPreference.target_kind': NotificationTargetKind.fromCode,
  'Thread.kind': ThreadKind.fromCode,
  'ThreadState': ThreadState.fromCode,
  'Thread.title_source': TitleSource.fromCode,
  'MessageKind': MessageKind.fromCode,
  'MessageOrigin': MessageOrigin.fromCode,
  'TriggerState': TriggerState.fromCode,
  'Message.system_code': SystemCode.fromCode,
  'File.purpose': FilePurpose.fromCode,
  'File.upload_state': UploadState.fromCode,
  'File.scan_status': ScanStatus.fromCode,
  'TranscriptStatus': TranscriptStatus.fromCode,
  'TranscriptSource': TranscriptSource.fromCode,
  'Thumbnail.variant': ThumbnailVariant.fromCode,
  'ComponentType': ComponentType.fromCode,
  'ComponentRestriction': ComponentRestriction.fromCode,
  'Component.state': ComponentState.fromCode,
  'Capability': Capability.fromCode,
  'RunState': RunState.fromCode,
  'Run.trigger_kind': RunTriggerKind.fromCode,
  'RunStep.kind': StepKind.fromCode,
  'RunStep.status': StepStatus.fromCode,
  'PrincipalKind': PrincipalKind.fromCode,
  'UserStatus': PrincipalStatus.fromCode,
  'BillingMode': BillingMode.fromCode,
  'ToolClass': ToolClass.fromCode,
  'AgentRuntimeState': AgentRuntimeState.fromCode,
  'TargetKind': TargetKind.fromCode,
  'CommandLevel': CommandLevel.fromCode,
  'CommandOption.type': CommandOptionType.fromCode,
};

List<String> _specValues(YamlMap schemas, String ref) {
  final parts = ref.split('.');
  var node = schemas[parts.first] as YamlMap?;
  if (node == null) fail('schema ${parts.first} is missing from openapi.yaml');
  if (parts.length == 2) {
    node = (node['properties'] as YamlMap?)?[parts[1]] as YamlMap?;
    if (node == null) fail('property $ref is missing from openapi.yaml');
  }
  final values = node['enum'] as YamlList?;
  if (values == null) fail('$ref has no enum in openapi.yaml');
  return [for (final v in values) '$v'];
}

void main() {
  final doc = loadYaml(readRepoFile('docs/api/openapi.yaml')) as YamlMap;
  final schemas = (doc['components'] as YamlMap)['schemas'] as YamlMap;

  test('every enum value of the spec schemas has a member', () {
    for (final entry in _enums.entries) {
      final codes = {for (final m in entry.value) m.code};
      for (final value in _specValues(schemas, entry.key)) {
        expect(codes, contains(value), reason: '${entry.key}: $value');
      }
    }
  });

  test('every enum has an unknown fallback and decodes its own codes', () {
    for (final entry in _decoders.entries) {
      final decode = entry.value;
      expect((decode('rocket') as Enum).name, 'unknown', reason: entry.key);
      for (final member in _enums[entry.key]!) {
        if (member.code.isNotEmpty) {
          expect(decode(member.code), member, reason: entry.key);
        }
      }
    }
  });

  test('fromCode of a new value is unknown', () {
    expect(ChannelKind.fromCode('rocket'), ChannelKind.unknown);
    expect(TriggerState.fromCode(''), TriggerState.unknown);
    expect(AgentRuntimeState.fromCode('rocket'), AgentRuntimeState.unknown);
  });
}
