// SPDX-License-Identifier: Apache-2.0

// Accessibility of the search (brief M4-32 Design note 1; 01 5.1 and 8; components.md 12): the
// field and the filters are labelled, the filter chips say whether they are on, every hit reads
// its place and text, the count of hits is text, and the local-only note is said in words.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/search/search_view_golden_test.dart' as golden;

void main() {
  for (final MapEntry(key: name, value: view) in golden.views.entries) {
    dsAccessibilityTests(
      'the search $name',
      build: view,
      semantics: (tester, l) {
        expect(
          tester
              .getSemantics(find.byKey(const ValueKey('search.field')))
              .getSemanticsData()
              .flagsCollection
              .isTextField,
          isTrue,
        );
      },
      reachable: (l) => [
        find.byKey(const ValueKey('search.field')),
        find.byKey(const ValueKey('search.submit')),
      ],
    );
  }
}
