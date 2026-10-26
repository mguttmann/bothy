// SPDX-License-Identifier: Apache-2.0

// The child sessions of the app extensions on the fake (brief M5-06; 08 7.4): the app creates
// one with `createExtensionSession`, it has its own refresh family on the same device key, a
// new child of the same component replaces the old one, and revoking the parent revokes it.

import 'package:api_client/api_client.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  Future<TokenResponse> createChild(TestDevice device) async {
    final response = await device.api.getDevicesApi().createExtensionSession(
      extensionSessionCreate: ExtensionSessionCreate(
        (b) => b
          ..clientComponent =
              ExtensionSessionCreateClientComponentEnum.shareExt,
      ),
    );
    expect(response.statusCode, 201);
    return response.data!;
  }

  test('a child session has its own refresh family on the same key', () async {
    final alex = h.device();
    final parent = await signedIn(h, alex);

    final child = await createChild(alex);

    final world = h.server.world;
    final stored = world.sessions[child.session.id]!;
    final parentSession = world.sessions.values.firstWhere(
      (s) => s.accessToken == parent.token,
    );
    expect(child.session.clientComponent, SessionClientComponentEnum.shareExt);
    expect(stored.parentId, parentSession.id);
    expect(stored.jkt, parentSession.jkt);
    expect(child.refreshToken, isNot(parentSession.refreshToken));
    expect(h.server.violations, isEmpty);
  });

  test('a new child of the same component replaces the old one', () async {
    final alex = h.device();
    await signedIn(h, alex);

    final first = await createChild(alex);
    final second = await createChild(alex);

    final world = h.server.world;
    expect(world.sessions[first.session.id]!.revoked, isTrue);
    expect(world.sessions[second.session.id]!.revoked, isFalse);
  });

  test('revoking the parent revokes the child', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final child = await createChild(alex);
    final world = h.server.world;
    final parentId = world.sessions[child.session.id]!.parentId!;

    world.revokeSession(parentId);

    expect(world.sessions[child.session.id]!.revoked, isTrue);
  });
}
