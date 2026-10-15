// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('personal: icon, badge colors, text and label', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsBillingBadge(mode: DsBillingMode.personal, holderName: 'Alex'),
    );
    expect(find.text(l.billingPersonal), findsOneWidget);
    expect(
      find.bySemanticsLabel(l.billingLabelPersonal('Alex')),
      findsOneWidget,
    );
    final icon = tester.widget<DsIcon>(find.byType(DsIcon));
    expect(icon.glyph, DsIcons.billingPersonal);
    expect(icon.color, DsColors.light.badgePersonalFg);
  });

  testWidgets('organization: icon, text and label', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsBillingBadge(mode: DsBillingMode.organization),
    );
    expect(find.text(l.billingOrganization), findsOneWidget);
    expect(find.bySemanticsLabel(l.billingLabelOrganization), findsOneWidget);
    final icon = tester.widget<DsIcon>(find.byType(DsIcon));
    expect(icon.glyph, DsIcons.billingOrganization);
    expect(icon.color, DsColors.light.badgeOrganizationFg);
  });

  testWidgets('a differing reported mode adds the warning badge', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsBillingBadge(
        mode: DsBillingMode.personal,
        holderName: 'Alex',
        reportedMode: DsBillingMode.organization,
      ),
    );
    expect(find.text(l.billingReported(l.billingOrganization)), findsOneWidget);
    expect(
      tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
      contains(DsIcons.stateWarning),
    );

    await pumpDs(
      tester,
      const DsBillingBadge(
        mode: DsBillingMode.organization,
        reportedMode: DsBillingMode.organization,
      ),
    );
    expect(find.text(l.billingReported(l.billingOrganization)), findsNothing);
  });

  testWidgets('an unknown mode shows its code', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsBillingBadge(mode: DsBillingMode.unknown, code: 'future_mode'),
    );
    expect(find.text(l.billingUnknown('future_mode')), findsOneWidget);
  });

  testWidgets(
    'cost text: Estimate for organization, information only for personal',
    (tester) async {
      final l = await en();
      await pumpDs(
        tester,
        const DsCostText(
          DsCost(
            DsMoney(1.2, 'USD'),
            mode: DsBillingMode.organization,
            isEstimate: true,
          ),
        ),
      );
      expect(allText(tester), contains('USD 1.20'));
      expect(allText(tester), contains(l.costEstimate));

      await pumpDs(
        tester,
        const DsCostText(
          DsCost(DsMoney(1.2, 'USD'), mode: DsBillingMode.organization),
        ),
      );
      expect(allText(tester), isNot(contains(l.costEstimate)));

      await pumpDs(
        tester,
        const DsCostText(
          DsCost(DsMoney(1.2, 'USD'), mode: DsBillingMode.personal),
        ),
      );
      expect(allText(tester), contains(l.costInformationOnly));

      final g = await de();
      await pumpDs(
        tester,
        const DsCostText(
          DsCost(
            DsMoney(1.2, 'USD'),
            mode: DsBillingMode.organization,
            isEstimate: true,
          ),
        ),
        locale: const Locale('de'),
      );
      expect(allText(tester), contains('1,20 USD'));
      expect(allText(tester), contains(g.costEstimate));
    },
  );
}
