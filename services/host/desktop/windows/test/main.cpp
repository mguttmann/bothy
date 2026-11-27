// SPDX-License-Identifier: Apache-2.0

// Entry point of helper_core_tests; the tests register themselves through CHECK_TEST.

#include "check.hpp"

int main(int argc, char** argv) { return check::run(argc, argv); }
