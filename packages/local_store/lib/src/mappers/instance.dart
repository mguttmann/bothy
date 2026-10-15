// SPDX-License-Identifier: Apache-2.0

/// Rows of the instance and of commands to domain entities and back.
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';

/// The insert companion of [info].
InstanceTableCompanion instanceToCompanion(InstanceInfo info) =>
    InstanceTableCompanion.insert(
      instanceId: info.instanceId,
      productName: info.branding.productName,
      accentColor: Value(info.branding.accentColor),
      logoFileId: Value(info.branding.logoFileId?.value),
      brandingVersion: info.branding.version.value,
      localeDefault: info.localeDefault,
      specVersion: info.specVersion,
      features: jsonEncode(info.features),
      limits: jsonEncode({
        'message_max_content_bytes': info.limits.messageMaxContentBytes,
        'message_max_attachments': info.limits.messageMaxAttachments,
        'file_max_upload_bytes': info.limits.fileMaxUploadBytes,
        'thread_title_max_chars': info.limits.threadTitleMaxChars,
        'voice_max_duration_ms': info.limits.voiceMaxDurationMs,
      }),
    );

/// The instance information of [row].
InstanceInfo instanceFromRow(InstanceRow row) {
  final limits = jsonDecode(row.limits) as Map<String, Object?>;
  return InstanceInfo(
    instanceId: row.instanceId,
    branding: Branding(
      productName: row.productName,
      accentColor: row.accentColor,
      logoFileId: row.logoFileId == null ? null : FileId.parse(row.logoFileId!),
      version: ResourceVersion(row.brandingVersion),
    ),
    localeDefault: row.localeDefault,
    specVersion: row.specVersion,
    features: Map.unmodifiable(
      (jsonDecode(row.features) as Map).cast<String, bool>(),
    ),
    limits: InstanceLimits(
      messageMaxContentBytes: limits['message_max_content_bytes']! as int,
      messageMaxAttachments: limits['message_max_attachments']! as int,
      fileMaxUploadBytes: limits['file_max_upload_bytes']! as int,
      threadTitleMaxChars: limits['thread_title_max_chars']! as int,
      voiceMaxDurationMs: limits['voice_max_duration_ms']! as int,
    ),
  );
}

/// The insert companion of [command].
CommandsCompanion commandToCompanion(Command command) =>
    CommandsCompanion.insert(
      id: command.id.value,
      ownerPrincipalId: command.ownerPrincipalId.value,
      name: command.name,
      description: command.description,
      scopeKind: command.scopeKind.code,
      scopeId: Value(command.scopeId),
      options: jsonEncode([
        for (final o in command.options)
          {
            'name': o.name,
            'description': o.description,
            'type': o.type.code,
            'required': o.required,
            'choices': [
              for (final c in o.choices) {'name': c.name, 'value': c.value},
            ],
            'autocomplete': o.autocomplete,
          },
      ]),
      requiredLevel: command.requiredLevel.code,
      control: command.control,
      version: command.version.value,
    );

/// The command of [row].
Command commandFromRow(CommandRow row) => Command(
  id: CommandId.parse(row.id),
  ownerPrincipalId: PrincipalId.parse(row.ownerPrincipalId),
  name: row.name,
  description: row.description,
  scopeKind: TargetKind.fromCode(row.scopeKind),
  scopeId: row.scopeId,
  options: List.unmodifiable([
    for (final o in jsonDecode(row.options) as List)
      CommandOption(
        name: (o as Map)['name'] as String,
        description: o['description'] as String,
        type: CommandOptionType.fromCode(o['type'] as String),
        required: o['required'] as bool,
        choices: List.unmodifiable([
          for (final c in o['choices'] as List)
            CommandChoice(
              name: (c as Map)['name'] as String,
              value: c['value'] as String,
            ),
        ]),
        autocomplete: o['autocomplete'] as bool,
      ),
  ]),
  requiredLevel: CommandLevel.fromCode(row.requiredLevel),
  control: row.control,
  version: ResourceVersion(row.version),
);
