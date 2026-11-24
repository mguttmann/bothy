// SPDX-License-Identifier: Apache-2.0

// The name a device registers with (08 5.1; AUD-PKG-P02-client-b): the runner's model where it
// reports one, the host name elsewhere, and never `localhost`, which iOS gives as its host name.

import 'package:client/app/bootstrap.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the runner\'s name wins', () {
    expect(
      registrationDeviceName(
        runner: 'iPhone (iPhone15,2)',
        hostName: 'localhost',
        platform: 'ios',
      ),
      'iPhone (iPhone15,2)',
    );
    expect(
      registrationDeviceName(
        runner: 'Google Pixel 7',
        hostName: 'localhost',
        platform: 'android',
      ),
      'Google Pixel 7',
    );
  });

  test('the desktop registers with its host name', () {
    expect(
      registrationDeviceName(
        runner: null,
        hostName: 'alex-laptop',
        platform: 'macos',
      ),
      'alex-laptop',
    );
  });

  test('an iPhone without the runner\'s answer is no "localhost"', () {
    expect(
      registrationDeviceName(
        runner: null,
        hostName: 'localhost',
        platform: 'ios',
      ),
      'iPhone or iPad',
    );
    expect(
      registrationDeviceName(runner: ' ', hostName: '', platform: 'ios'),
      'iPhone or iPad',
    );
  });
}
