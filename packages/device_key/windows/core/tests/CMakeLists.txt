# SPDX-License-Identifier: Apache-2.0
#
# The CTest suite of device_key_core: one executable, one CTest test per name. Exit code 77 is a
# skip with its reason in the output (tests/check.h, D-M4-11-4).
add_executable(device_key_core_tests main.cpp device_key_store_test.cpp)
target_link_libraries(device_key_core_tests PRIVATE device_key_core)
if(MSVC)
  target_compile_definitions(device_key_core_tests PRIVATE _HAS_EXCEPTIONS=0)
  target_compile_options(device_key_core_tests PRIVATE /W4 /WX /EHsc)
endif()

foreach(test_name IN ITEMS
    creates_two_distinct_keys_per_scope
    scopes_are_separated
    signature_is_raw_and_verifies
    tpm_path_when_available
    software_fallback_without_tpm
    keys_are_not_exportable
    step_up_binding
    existing_key_is_not_recreated_in_other_provider
    marker_without_key_is_invalidated
    delete_removes_both_keys
    errors_carry_no_key_material)
  add_test(NAME ${test_name} COMMAND device_key_core_tests ${test_name})
  set_tests_properties(${test_name} PROPERTIES SKIP_RETURN_CODE 77)
endforeach()
