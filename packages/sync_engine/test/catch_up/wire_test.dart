// SPDX-License-Identifier: Apache-2.0

// The wire mappers of the objects that arrive as event payloads and REST responses (04 2.2,
// 04 9 item 3): every member of the API shape reaches the domain type, optional members may be
// absent, and a missing required member is a FormatException.

import 'package:domain/domain.dart';
import 'package:sync_engine/src/wire.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  group('helpers', () {
    test('a missing or mistyped member is a FormatException', () {
      final j = <String, Object?>{'n': 'x', 's': 1, 'l': 'x', 'o': 'x'};
      expect(() => obj(j, 'o'), throwsFormatException);
      expect(() => optObj(j, 'o'), throwsFormatException);
      expect(() => objList(j, 'l'), throwsFormatException);
      expect(() => str(j, 's'), throwsFormatException);
      expect(() => optStr(j, 's'), throwsFormatException);
      expect(() => integer(j, 'n'), throwsFormatException);
      expect(() => optInt(j, 'n'), throwsFormatException);
      expect(() => flag(j, 'n'), throwsFormatException);
      expect(() => strList(j, 'l'), throwsFormatException);
    });

    test('optional members may be absent', () {
      final j = <String, Object?>{
        'o': {'a': 1},
      };
      expect(optObj(j, 'missing'), isNull);
      expect(optObj(j, 'o'), {'a': 1});
      expect(flag(j, 'missing', orElse: true), isTrue);
    });
  });

  test(
    'the instance maps its app lock policy, absent from an older server',
    () {
      final info = instanceFromJson(instanceJson());
      expect(
        info.appLock,
        const AppLockPolicy(required: false, afterMs: 300000),
      );
      final required = instanceFromJson({
        ...instanceJson(),
        'app_lock': {'required': true, 'after_ms': 1000},
      });
      expect(
        required.appLock,
        const AppLockPolicy(required: true, afterMs: 1000),
      );
      expect(
        instanceFromJson({...instanceJson()}..remove('app_lock')).appLock,
        isNull,
      );
      expect(
        () => instanceFromJson({
          ...instanceJson(),
          'app_lock': {'required': true},
        }),
        throwsFormatException,
      );
    },
  );

  test('a message component maps every member', () {
    final c = componentFromJson({
      'id': id(0x601),
      'custom_id': 'approve',
      'type': 'button',
      'definition': {'label': 'Approve'},
      'position': 2,
      'single_use': true,
      'required_capability': 'message.write',
      'restricted_to': 'triggerer',
      'run_id': id(0x701),
      'state': 'used',
      'expires_at': at,
      'used_by': samId,
      'used_at': at,
      'version': 3,
    }, MessageId.parse(messageId(1, 1)));
    expect(c.id, ComponentId.parse(id(0x601)));
    expect(c.messageId, MessageId.parse(messageId(1, 1)));
    expect(c.customId, 'approve');
    expect(c.type, ComponentType.fromCode('button'));
    expect(c.definition, {'label': 'Approve'});
    expect(c.position, 2);
    expect(c.singleUse, isTrue);
    expect(c.requiredCapability, Capability.fromCode('message.write'));
    expect(c.restrictedTo, ComponentRestriction.fromCode('triggerer'));
    expect(c.runId, RunId.parse(id(0x701)));
    expect(c.state, ComponentState.fromCode('used'));
    expect(c.expiresAt, DateTime.parse(at));
    expect(c.usedBy, PrincipalId.parse(samId));
    expect(c.usedAt, DateTime.parse(at));
    expect(c.version, const ResourceVersion(3));
  });

  test('file metadata maps every member and its thumbnails', () {
    final f = fileFromJson({
      'id': id(0x801),
      'sha256': 'ab' * 32,
      'size_bytes': 2048,
      'mime_type': 'image/png',
      'original_name': 'plan.png',
      'uploaded_by': samId,
      'purpose': 'attachment',
      'scope_channel_id': channelId(1),
      'upload_state': 'complete',
      'scan_status': 'clean',
      'width': 640,
      'height': 480,
      'duration_ms': 1500,
      'transcript_status': 'none',
      'transcript_language': 'en',
      'transcript_source': 'server',
      'thumbnails': [
        {
          'variant': 'small',
          'mime_type': 'image/webp',
          'width': 64,
          'height': 48,
        },
      ],
      'created_at': at,
    });
    expect(f.id, FileId.parse(id(0x801)));
    expect(f.sha256, 'ab' * 32);
    expect(f.sizeBytes, 2048);
    expect(f.mimeType, 'image/png');
    expect(f.originalName, 'plan.png');
    expect(f.uploadedBy, PrincipalId.parse(samId));
    expect(f.purpose, FilePurpose.fromCode('attachment'));
    expect(f.scopeChannelId, ChannelId.parse(channelId(1)));
    expect(f.uploadState, UploadState.fromCode('complete'));
    expect(f.scanStatus, ScanStatus.fromCode('clean'));
    expect([f.width, f.height, f.durationMs], [640, 480, 1500]);
    expect(f.transcriptStatus, TranscriptStatus.fromCode('none'));
    expect(f.transcriptLanguage, 'en');
    expect(f.transcriptSource, TranscriptSource.fromCode('server'));
    expect(f.thumbnails, hasLength(1));
    expect(f.thumbnails.single.variant, ThumbnailVariant.fromCode('small'));
    expect(f.thumbnails.single.mimeType, 'image/webp');
    expect([f.thumbnails.single.width, f.thumbnails.single.height], [64, 48]);
    expect(f.createdAt, DateTime.parse(at));
  });

  test('a user and an agent map to the principal and the agent card', () {
    final u = userFromJson({
      'id': samId,
      'handle': 'sam',
      'display_name': 'Sam',
      'avatar_file_id': id(0x802),
    });
    expect(u.id, PrincipalId.parse(samId));
    expect(u.kind, PrincipalKind.fromCode('user'));
    expect([u.handle, u.displayName], ['sam', 'Sam']);
    expect(u.avatarFileId, FileId.parse(id(0x802)));

    final a = agentFromJson({
      'id': robinId,
      'handle': 'robin',
      'display_name': 'Robin',
      'description': 'Builds things',
      'billing_mode': 'api',
      'tool_class': 'coding',
      'presence': {'state': 'idle', 'reported_billing_mode': 'api'},
      'version': 4,
    });
    expect(a.id, PrincipalId.parse(robinId));
    expect(
      [a.handle, a.displayName, a.description],
      ['robin', 'Robin', 'Builds things'],
    );
    expect(a.avatarFileId, isNull);
    expect(a.billingMode, BillingMode.fromCode('api'));
    expect(a.toolClass, ToolClass.fromCode('coding'));
    expect(a.presence.state, AgentRuntimeState.fromCode('idle'));
    expect(a.presence.reportedBillingMode, BillingMode.fromCode('api'));
    expect(a.version, const ResourceVersion(4));
  });

  test('a run summary maps every member with its cost', () {
    final r = runFromJson({
      'id': id(0x701),
      'thread_id': threadId(1),
      'channel_id': channelId(1),
      'agent_id': robinId,
      'triggered_by': alexId,
      'state': 'completed',
      'state_reason': 'done',
      'version': 5,
      'billing_mode_expected': 'api',
      'billing_mode_used': 'api',
      'status_line': 'Finished',
      'summary': 'All good',
      'cost': {'amount_micros': 1200, 'currency': 'EUR', 'is_estimate': true},
      'step_count': 3,
      'duration_ms': 4000,
      'queued_at': at,
      'started_at': at,
      'finished_at': at,
    });
    expect(r.id, RunId.parse(id(0x701)));
    expect(r.threadId, ThreadId.parse(threadId(1)));
    expect(r.channelId, ChannelId.parse(channelId(1)));
    expect(r.agentId, PrincipalId.parse(robinId));
    expect(r.triggeredBy, PrincipalId.parse(alexId));
    expect(r.state, RunState.fromCode('completed'));
    expect(r.stateReason, 'done');
    expect(r.version, const ResourceVersion(5));
    expect(r.billingModeExpected, BillingMode.fromCode('api'));
    expect(r.billingModeUsed, BillingMode.fromCode('api'));
    expect([r.statusLine, r.summary], ['Finished', 'All good']);
    expect(r.cost.amountMicros, 1200);
    expect(r.cost.currency, 'EUR');
    expect(r.cost.isEstimate, isTrue);
    expect([r.stepCount, r.durationMs], [3, 4000]);
    expect(r.queuedAt, DateTime.parse(at));
    expect(r.startedAt, DateTime.parse(at));
    expect(r.finishedAt, DateTime.parse(at));
  });

  test('a command maps its options and choices', () {
    final c = commandFromJson({
      'id': id(0x901),
      'owner_principal_id': robinId,
      'name': 'deploy',
      'description': 'Deploy a branch',
      'scope_kind': 'channel',
      'scope_id': channelId(1),
      'options': [
        {
          'name': 'target',
          'description': 'Where to',
          'type': 'string',
          'required': true,
          'choices': [
            {'name': 'Staging', 'value': 'staging'},
          ],
          'autocomplete': true,
        },
      ],
      'required_level': 'write',
      'control': true,
      'version': 2,
    });
    expect(c.id, CommandId.parse(id(0x901)));
    expect(c.ownerPrincipalId, PrincipalId.parse(robinId));
    expect([c.name, c.description], ['deploy', 'Deploy a branch']);
    expect(c.scopeKind, TargetKind.fromCode('channel'));
    expect(c.scopeId, channelId(1));
    final o = c.options.single;
    expect([o.name, o.description], ['target', 'Where to']);
    expect(o.type, CommandOptionType.fromCode('string'));
    expect(o.required, isTrue);
    expect(o.autocomplete, isTrue);
    expect(o.choices.single.name, 'Staging');
    expect(o.choices.single.value, 'staging');
    expect(c.requiredLevel, CommandLevel.fromCode('write'));
    expect(c.control, isTrue);
    expect(c.version, const ResourceVersion(2));
  });

  test(
    'sidebar state, notification preference and quiet hours map every member',
    () {
      final s = sidebarFromJson({
        'space_id': spaceId(1),
        'collapsed': true,
        'sort_key': 'b',
        'version': 2,
        'updated_by_device_id': otherDevice,
      });
      expect(s.spaceId, SpaceId.parse(spaceId(1)));
      expect(s.collapsed, isTrue);
      expect(s.sortKey, 'b');
      expect(s.version, const ResourceVersion(2));
      expect(s.updatedByDeviceId, DeviceId.parse(otherDevice));

      final p = preferenceFromJson({
        'target_kind': 'channel',
        'target_id': channelId(1),
        'level': 'mentions',
        'muted_until': at,
        'version': 3,
      });
      expect(p.targetKind, NotificationTargetKind.fromCode('channel'));
      expect(p.targetId, channelId(1));
      expect(p.level, NotificationLevel.fromCode('mentions'));
      expect(p.mutedUntil, DateTime.parse(at));
      expect(p.version, const ResourceVersion(3));

      final q = quietHoursFromJson({
        'id': id(0xa01),
        'weekdays': 31,
        'start_local': '22:00',
        'end_local': '07:00',
        'time_zone': 'Europe/Berlin',
        'enabled': true,
        'version': 1,
      });
      expect(q.id, QuietHoursId.parse(id(0xa01)));
      expect(q.weekdays, 31);
      expect(
        [q.startLocal, q.endLocal, q.timeZone],
        ['22:00', '07:00', 'Europe/Berlin'],
      );
      expect(q.enabled, isTrue);
      expect(q.version, const ResourceVersion(1));
    },
  );
}
