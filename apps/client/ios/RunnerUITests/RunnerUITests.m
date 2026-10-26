// SPDX-License-Identifier: Apache-2.0
//
// The bridge of the patrol device tests on iOS (brief M5-06 Tests): `patrol test` builds the app
// with a bundle of the Dart tests as its entry point, and this XCTest class lists them and runs
// each one as a test method of its own, so the tests can also drive the system share sheet,
// Photos and the Share Extension through XCUITest.
@import XCTest;
@import patrol;
@import ObjectiveC.runtime;

PATROL_INTEGRATION_TEST_IOS_RUNNER(RunnerUITests)
