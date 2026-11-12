// SPDX-License-Identifier: Apache-2.0

/// One person as an entry of a choice between people (docs/spec/03-data-model.md `users`:
/// only the handle is unique; ruling AUD-AM1/4).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';

import '../data/admin_repository.dart';

/// The display name of [person] with the handle after it (`Sam @sam`), as the people list
/// shows a person.
///
/// Two people can carry the same display name, so a choice between people that shows only the
/// name cannot tell them apart; every dropdown that picks a person uses this entry ("Add owner",
/// "Add member", "View as", the person filter of the audit log).
///
/// Example: `DropdownMenuItem(value: p, child: PersonChoice(person: p))`.
class PersonChoice extends StatelessWidget {
  /// Creates the entry of [person].
  const PersonChoice({super.key, required this.person});

  /// The person.
  final AdminPerson person;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(person.displayName, overflow: TextOverflow.ellipsis),
        ),
        const SizedBox(width: DsSpace.s8),
        Flexible(
          child: Text(
            '@${person.handle}',
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
        ),
      ],
    );
  }
}
