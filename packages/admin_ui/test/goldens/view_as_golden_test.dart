// SPDX-License-Identifier: Apache-2.0

// Goldens of the stored view in the console (brief M6-04 Tests): the banner and the structure
// at a browser width of 1280 logical pixels; light and dark, text scaling 1 and 2, German and
// English. One file per view keeps each file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(width: 1280, child: child),
);

const ViewAsItem _marketing = ViewAsItem(
  id: 'marketing',
  name: 'Marketing',
  level: AccessLevel.read,
  capabilities: [],
);

final ViewAsResult _snapshot = ViewAsResult(
  user: const AccessPrincipal(
    id: 'sam',
    kind: 'user',
    handle: 'sam',
    displayName: 'Sam',
  ),
  expiresAt: DateTime.utc(2026, 10, 17, 9, 15),
  spaces: const [_marketing],
  channels: const [
    ViewAsItem(
      id: 'campaigns',
      name: 'Kampagnen',
      level: AccessLevel.trigger,
      capabilities: [AccessCapability.upload],
      spaceId: 'marketing',
    ),
    ViewAsItem(
      id: 'internal',
      name: 'Intern',
      level: AccessLevel.read,
      capabilities: [],
      spaceId: 'marketing',
    ),
  ],
);

final Map<String, Widget Function()> _views = {
  'banner': () => _frame(
    ViewAsBanner(
      name: 'Sam',
      endsAt: testNow.add(const Duration(minutes: 14, seconds: 5)),
      now: testNow,
      onEnd: () {},
    ),
  ),
  'structure': () => _frame(
    Padding(
      padding: const EdgeInsets.all(24),
      child: ViewAsStructure(
        name: 'Sam',
        snapshot: _snapshot,
        notVisible: const ['Board', 'Sales'],
      ),
    ),
  ),
  'structure_later': () => _frame(
    const Padding(
      padding: EdgeInsets.all(24),
      child: ViewAsStructure(name: 'Sam', snapshot: null, notVisible: []),
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name at 1280 in light and dark, text scale 1 and 2, de and en',
      fileName: 'view_as_$name',
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
