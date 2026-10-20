# shellcheck shell=sh
# SPDX-License-Identifier: Apache-2.0
#
# Cases of the install.sh tests, sourced by run.sh (helpers and variables in upper case are
# defined there). Each test_* function runs in a subshell in its own empty folder $CASE; the
# default target folder is $CASE/bothy. Exit code 0 passes, 77 skips (the last log line is the
# reason), anything else fails. Every case also fails when install.sh called sudo (run.sh).

# std_env prints the path of the .env of the default target folder.
std_env() {
	printf '%s\n' "$CASE/bothy/infra/compose/.env"
}

# expect_nothing_written fails unless the work folder equals the snapshot $1 and the target
# folder does not exist.
expect_nothing_written() {
	[ ! -e "$CASE/bothy" ] || {
		show
		fail "the target folder was created"
	}
	[ "$(snapshot)" = "$1" ] || {
		show
		snapshot >"$CASE.after"
		printf '%s\n' "$1" | diff - "$CASE.after"
		fail "the run created or removed files"
	}
}

# unchanged_lines prints a file without the lines install.sh may change, for a byte comparison
# of the rest. Arguments: $1 file.
unchanged_lines() {
	grep -v -e '^COMPOSE_PROFILES=' -e '^BOTHY_PUBLIC_URL=' -e '^BOTHY_STORAGE_DRIVER=' \
		-e '^BOTHY_BACKUP_OFFSITE_TARGET=' -e '^BOTHY_TRUSTED_PROXIES=' -e '^TS_AUTHKEY=' "$1"
}

# ---- static checks of the script ------------------------------------------------------------

test_script_static() {
	[ -f "$SOURCE_SCRIPT" ] || fail "$SOURCE_SCRIPT is missing"
	[ "$(head -n 1 "$SOURCE_SCRIPT")" = '#!/bin/sh' ] || fail "the first line is not #!/bin/sh"
	sh -n "$SOURCE_SCRIPT" || fail "sh -n rejects the script"
	for key in BOTHY_INSTALL_VERSION BOTHY_INSTALL_BUNDLE_SHA256 BOTHY_INSTALL_BASE_URL; do
		count=$(grep -c "^$key='[^']*'\$" "$SOURCE_SCRIPT")
		[ "$count" = 1 ] || fail "$key= is there $count times, expected once"
	done
	# The release build aborts when the rendered script still carries a marker of the source.
	if grep -n -i 'placeholder' "$INSTALL" || grep -n 'OWNER' "$INSTALL"; then
		fail "the rendered script still contains OWNER or placeholder"
	fi
	code=$(sed -e 's/^[[:space:]]*#.*//' "$SOURCE_SCRIPT")
	if printf '%s\n' "$code" | grep -n -w 'sudo'; then
		fail "the script calls sudo"
	fi
	# Cheap detection only: bash keywords in command position, [[ ]], here-strings, $'...',
	# == inside [ ], &> and arrays. ShellCheck -s sh in the gate is the full check.
	if printf '%s\n' "$code" | grep -n -E '^[[:space:]]*(local|function|declare|typeset|source|shopt)[[:space:]]|\[\[ |<<<|\$'\''|\[ [^]]* == |&>|[a-z_]+=\(|\$\{[a-z_]+\['; then
		fail "the script contains a bashism"
	fi
	if grep -n -E "$(printf '\342\200[\223\224]')" "$SOURCE_SCRIPT"; then
		fail "the script contains an en dash or an em dash"
	fi
}

# ---- options --------------------------------------------------------------------------------

test_help_exits_0() {
	inst sh "$INSTALL" --help
	expect_rc 0
	expect_in "$OUT" "--public-url"
	expect_in "$OUT" "--ready-timeout"
	[ ! -e "$CASE/bothy" ] || fail "--help created the target folder"
}

test_usage_errors_exit_2() {
	inst sh "$INSTALL" --bogus
	expect_rc 2
	expect_in "$ERR" "install.sh: options: "
	expect_in "$ERR" "--bogus"
	inst sh "$INSTALL" --edge=sideways
	expect_rc 2
	expect_in "$ERR" "install.sh: options: --edge"
	inst sh "$INSTALL" --ready-timeout 0
	expect_rc 2
	expect_in "$ERR" "install.sh: options: --ready-timeout"
	inst sh "$INSTALL" --owner-handle Alex
	expect_rc 2
	expect_in "$ERR" "install.sh: options: --owner-handle"
	inst sh "$INSTALL" --public-url ftp://chat.example.com
	expect_rc 2
	expect_in "$ERR" "install.sh: options: --public-url"
	inst sh "$INSTALL" --dir
	expect_rc 2
	expect_in "$ERR" "install.sh: options: --dir"
	[ ! -e "$CASE/bothy" ] || fail "a usage error created the target folder"
}

# ---- preflight ------------------------------------------------------------------------------

test_preflight_docker_missing() {
	before=$(snapshot)
	inst PATH="$NODOCKER_PATH" sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: preflight: "
	expect_in "$ERR" "docker"
	expect_in "$ERR" "https://docs.docker.com/"
	expect_nothing_written "$before"
}

test_preflight_engine_25() {
	before=$(snapshot)
	inst STUB_ENGINE_VERSION=25.0.5 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: preflight: "
	expect_in "$ERR" "25.0.5"
	expect_nothing_written "$before"
}

test_preflight_compose_2_25() {
	before=$(snapshot)
	inst STUB_COMPOSE_VERSION=v2.25.0-desktop.1 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: preflight: "
	expect_in "$ERR" "Compose"
	expect_nothing_written "$before"
}

test_preflight_target_not_empty() {
	mkdir "$CASE/bothy" && printf 'mine\n' >"$CASE/bothy/notes.txt"
	before=$(snapshot)
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: preflight: "
	[ "$(snapshot)" = "$before" ] || fail "the run wrote into a foreign folder"
}

# ---- bundle ---------------------------------------------------------------------------------

test_bundle_checksum_mismatch_download() {
	inst STUB_DL_DIR="$WORK/dl-bad" sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: bundle: "
	expect_in "$ERR" "$GOOD_SHA"
	expect_in "$ERR" "$(sha256_of "$WORK/dl-bad/$TEST_BUNDLE_NAME")"
	[ ! -e "$(std_env)" ] || fail ".env was written"
	[ ! -e "$CASE/bothy/infra" ] || fail "the bundle was unpacked"
	if find "$CASE/bothy" -name '.install-tmp*' | grep . >/dev/null; then fail "the temp folder is left"; fi
}

test_bundle_checksum_mismatch_option() {
	inst sh "$INSTALL" --bundle "$WORK/dl-bad/$TEST_BUNDLE_NAME" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: bundle: "
	expect_in "$ERR" "$GOOD_SHA"
	if grep "^url " "$STUB_LOG"; then fail "install.sh downloaded although --bundle was given"; fi
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_bundle_option_installs_without_download() {
	inst sh "$INSTALL" --bundle "$WORK/dl/$TEST_BUNDLE_NAME" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	if grep "^url " "$STUB_LOG"; then fail "install.sh downloaded although --bundle was given"; fi
	expect_in "$STUB_LOG" "cosign verify-blob --bundle $WORK/dl/$TEST_BUNDLE_NAME.sigstore.json "
	expect_in "$STUB_LOG" " $WORK/dl/$TEST_BUNDLE_NAME"
	[ -f "$(std_env)" ] || fail ".env is missing"
}

test_bundle_signature_checked_with_cosign() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_in "$STUB_LOG" "url $TEST_BASE_URL$TEST_BUNDLE_NAME.sigstore.json"
	expect_in "$STUB_LOG" "--certificate-identity-regexp ^https://github\.com/example/bothy/\.github/workflows/release\.yml@refs/tags/v --certificate-oidc-issuer https://token.actions.githubusercontent.com "
}

test_bundle_signature_failure_aborts() {
	inst STUB_COSIGN_EXIT=1 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: bundle: "
	expect_in "$ERR" "signature"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_bundle_without_cosign_names_the_command() {
	inst PATH="$NOCOSIGN_PATH" sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_in "$ERR" "cosign verify-blob --bundle "
	expect_in "$ERR" "--certificate-identity-regexp '^https://github\.com/example/bothy/\.github/workflows/release\.yml@refs/tags/v' --certificate-oidc-issuer https://token.actions.githubusercontent.com"
	expect_not_in "$STUB_LOG" "cosign "
}

test_bundle_member_outside_top_directory() {
	inst STUB_DL_DIR="$WORK/dl-evil" sh "$INSTALL_EVIL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: bundle: "
	expect_in "$ERR" "other"
	[ ! -e "$CASE/bothy/other" ] || fail "the foreign member was unpacked"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_bundle_download_url() {
	inst STUB_ENGINE_VERSION=26.0.0 STUB_COMPOSE_VERSION=v2.26.0-desktop.1 STUB_ARCH=aarch64 \
		sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	urls=$(sed -n 's/^url //p' "$STUB_LOG")
	expected=$(printf '%s\n%s\n' "$TEST_BASE_URL$TEST_BUNDLE_NAME" "$TEST_BASE_URL$TEST_BUNDLE_NAME.sigstore.json")
	[ "$urls" = "$expected" ] || {
		show
		fail "downloads: $urls"
	}
	if find "$CASE/bothy" -name '.install-tmp*' | grep . >/dev/null; then fail "the temp folder is left"; fi
	[ "$(cat "$CASE/bothy/.bothy-install")" = "version=$TEST_VERSION" ] || fail "marker: $(cat "$CASE/bothy/.bothy-install")"
}

test_unrendered_source_aborts() {
	before=$(snapshot)
	inst sh "$SOURCE_SCRIPT" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: bundle: this is the unrendered source of install.sh; use the install.sh of a release"
	expect_nothing_written "$before"
}

# ---- prompts --------------------------------------------------------------------------------

test_no_terminal_missing_public_url() {
	inst sh "$INSTALL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: prompts: "
	expect_in "$ERR" "--public-url"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_no_terminal_missing_owner_handle() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: prompts: "
	expect_in "$ERR" "--owner-handle"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_no_terminal_missing_owner_name() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex
	expect_rc 1
	expect_in "$ERR" "install.sh: prompts: "
	expect_in "$ERR" "--owner-name"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_no_terminal_missing_ts_authkey() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --edge tailscale --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: prompts: "
	expect_in "$ERR" "TS_AUTHKEY"
	[ ! -e "$(std_env)" ] || fail ".env was written"
}

test_terminal_prompts() {
	if ! script --version 2>&1 | grep util-linux >/dev/null; then
		echo "script from util-linux is not available on this host"
		exit 77
	fi
	cmd="cd '$CASE' && env -i PATH='$TEST_PATH' HOME='$CASE' STUB_LOG='$STUB_LOG' STUB_DL_DIR='$WORK/dl'"
	cmd="$cmd STUB_TS_STATUS_EXIT=1 sh '$INSTALL' --ready-timeout 1s; echo \$? >'$CASE/tty.rc'"
	{
		printf '%s\n' "$PUBLIC_URL" tailscale tskey-auth-tty123 off s3 on \
			s3:https://s3.example.com/bothy-backup alex 'Alex Example'
		n=0
		while [ ! -e "$CASE/tty.rc" ] && [ "$n" -lt 120 ]; do
			sleep 1
			n=$((n + 1))
		done
	} | SHELL=/bin/sh script -qec "$cmd" /dev/null >"$OUT" 2>"$ERR"
	# shellcheck disable=SC2034 # read by expect_rc of run.sh
	RC=$(cat "$CASE/tty.rc" 2>/dev/null)
	expect_rc 0
	expect_in "$OUT" "Public address"
	expect_in "$OUT" "Enrollment code (shown once): $ENROLL_CODE"
	expect_env COMPOSE_PROFILES tailscale,s3,observability
	expect_env BOTHY_PUBLIC_URL "$PUBLIC_URL"
	expect_env BOTHY_STORAGE_DRIVER s3
	expect_env BOTHY_BACKUP_OFFSITE_TARGET s3:https://s3.example.com/bothy-backup
	expect_env TS_AUTHKEY tskey-auth-tty123
	expect_in "$STUB_LOG" "docker compose exec -T api-tailscale bothy setup bootstrap-owner --handle alex --display-name Alex Example"
}

# ---- write ----------------------------------------------------------------------------------

test_env_and_secrets_modes_tailscale() {
	inst TS_AUTHKEY=tskey-auth-env456 STUB_TS_STATUS_EXIT=1 \
		sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --edge tailscale --ready-timeout 1s --owner-handle alex --owner-name Alex
	expect_rc 0
	env_file=$(std_env)
	[ "$(mode_of "$env_file")" = "-rw-------" ] || fail ".env mode $(mode_of "$env_file")"
	[ "$(mode_of "$CASE/bothy/infra/compose/secrets")" = "drwx------" ] ||
		fail "secrets/ mode $(mode_of "$CASE/bothy/infra/compose/secrets")"
	expect_env TS_AUTHKEY tskey-auth-env456
	expect_env COMPOSE_PROFILES tailscale,scan
	# No other line than the chosen values differs from the template: no secret was added.
	[ "$(unchanged_lines "$env_file" | cksum)" = "$(unchanged_lines "$CASE/bothy/infra/compose/.env.example" | cksum)" ] ||
		fail "lines other than the answers changed"
	[ "$(wc -l <"$env_file")" = "$(wc -l <"$CASE/bothy/infra/compose/.env.example")" ] || fail "lines were added"
	expect_in "$STUB_LOG" "docker compose exec -T api-tailscale bothy healthcheck --url http://127.0.0.1:8081/readyz"
	expect_not_in "$STUB_LOG" "port api"
	expect_not_in "$STUB_LOG" "network inspect"
	expect_not_in "$STUB_LOG" "tskey-auth-env456"
	expect_in "$ERR" "TS_AUTHKEY"
	expect_in "$OUT" "Enrollment code (shown once): $ENROLL_CODE"
	if grep -r -l -F "$ENROLL_CODE" "$CASE/bothy"; then fail "the enrollment code is in a file of the target folder"; fi
}

test_tailscale_clears_authkey_after_login() {
	inst TS_AUTHKEY=tskey-auth-env789 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --edge tailscale --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env TS_AUTHKEY ""
	[ "$(mode_of "$(std_env)")" = "-rw-------" ] || fail ".env mode $(mode_of "$(std_env)")"
	expect_in "$STUB_LOG" "docker compose exec -T tailscale tailscale status"
	if grep -r -l -F tskey-auth-env789 "$CASE/bothy"; then fail "the auth key is left in a file"; fi
}

test_enrollment_code_only_on_output() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	[ "$(grep -c -F "$ENROLL_CODE" "$OUT")" = 1 ] || fail "the code is not shown exactly once"
	expect_in "$OUT" "Enrollment code (shown once): $ENROLL_CODE"
	expect_not_in "$ERR" "$ENROLL_CODE"
	if grep -r -l -F "$ENROLL_CODE" "$CASE/bothy"; then fail "the enrollment code is in a file of the target folder"; fi
	[ "$(mode_of "$(std_env)")" = "-rw-------" ] || fail ".env mode $(mode_of "$(std_env)")"
	[ "$(mode_of "$CASE/bothy/infra/compose/secrets")" = "drwx------" ] || fail "secrets/ mode"
}

test_storage_s3() {
	inst sh "$INSTALL" --yes --public-url="$PUBLIC_URL" --storage=s3 --owner-handle=alex --owner-name=Alex
	expect_rc 0
	expect_env COMPOSE_PROFILES direct,scan,s3
	expect_env BOTHY_STORAGE_DRIVER s3
}

test_scan_default_follows_memory() {
	inst STUB_MEMTOTAL=2147483648 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env COMPOSE_PROFILES direct
	inst STUB_MEMTOTAL=2147483648 sh "$INSTALL" --dir second --scan on --observability on --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env COMPOSE_PROFILES direct,scan,observability "$CASE/second/infra/compose/.env"
	expect_in "$ERR" "3 GiB"
}

test_bundle_versions_pass_through() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --offsite sftp:backup@backup.example.com:/bothy --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env BOTHY_VERSION "$SERVER_VERSION"
	expect_env BOTHY_REGISTRY "$REGISTRY"
	expect_env BOTHY_PUBLIC_URL "$PUBLIC_URL"
	expect_env BOTHY_STORAGE_DRIVER fs
	expect_env BOTHY_BACKUP_OFFSITE_TARGET sftp:backup@backup.example.com:/bothy
	[ "$(unchanged_lines "$(std_env)" | cksum)" = "$(unchanged_lines "$WORK/src/good/$TOP/infra/compose/.env.example" | cksum)" ] ||
		fail "lines other than the answers differ from the bundle's .env.example"
	expect_not_in "$OUT" "local backup only"
}

# ---- start ----------------------------------------------------------------------------------

test_start_order_and_finish_text() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	order=$(grep -E '^docker compose (config --quiet|up --no-start|up -d|exec -T api bothy healthcheck|exec -T api bothy setup)' "$STUB_LOG" | uniq | cut -d' ' -f3-7)
	expected=$(printf '%s\n' 'config --quiet' 'up --no-start' 'up -d' 'exec -T api bothy healthcheck' 'exec -T api bothy setup')
	[ "$order" = "$expected" ] || {
		show
		fail "order of the compose calls: $order"
	}
	compose_cwds=$(awk '/^docker compose / && !/^docker compose version/ { getline; sub(/^cwd /, ""); print }' "$STUB_LOG" | sort -u)
	[ "$compose_cwds" = "$CASE/bothy/infra/compose" ] || fail "docker ran outside the compose folder: $compose_cwds"
	expect_not_in "$STUB_LOG" " -p "
	expect_not_in "$STUB_LOG" "--project-name"
	expect_in "$OUT" "$PUBLIC_URL"
	expect_in "$OUT" "docker compose run --rm -T secrets-init secrets export --from /run/bothy/secrets/_store --age-recipient"
	expect_in "$OUT" "local backup only"
	expect_in "$OUT" "docker compose ps"
}

test_gateway_ipv4() {
	inst STUB_NETWORK=custom_net_7 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_in "$STUB_LOG" "docker network inspect --format {{range .IPAM.Config}}{{.Gateway}} {{end}} custom_net_7"
	expect_env BOTHY_TRUSTED_PROXIES 192.0.2.1/32
	[ "$(mode_of "$(std_env)")" = "-rw-------" ] || fail ".env mode $(mode_of "$(std_env)")"
	expect_in "$STUB_LOG" "curl -sS -o /dev/null -w %{http_code} http://127.0.0.1:8090/api/v1/instance/discovery"
}

test_gateway_ipv6() {
	inst STUB_GW6=2001:db8::1 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env BOTHY_TRUSTED_PROXIES 192.0.2.1/32,2001:db8::1/128
}

test_gateway_missing_aborts() {
	inst STUB_GW4= sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: start: "
	expect_not_in "$STUB_LOG" "docker compose up -d"
}

test_discovery_not_200() {
	inst STUB_DISCOVERY_STATUS=400 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: start: "
	expect_in "$ERR" "BOTHY_TRUSTED_PROXIES"
	expect_in "$ERR" "400"
	expect_not_in "$STUB_LOG" "bootstrap-owner"
}

test_ready_timeout() {
	inst STUB_READY_FAIL=1 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --ready-timeout 1s --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: start: "
	expect_in "$ERR" "install.ready_timeout"
	expect_in "$ERR" "docker compose logs api"
}

# ---- second run -----------------------------------------------------------------------------

test_second_run_keeps_env_and_secrets() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	env_file=$(std_env)
	secrets=$CASE/bothy/infra/compose/secrets
	# The operator sets the proxy and delivers a secret; the second run must keep both.
	if ! sed 's|^BOTHY_TRUSTED_PROXIES=.*|BOTHY_TRUSTED_PROXIES=198.51.100.7/32|' "$env_file" >"$CASE/env.new" ||
		! cat "$CASE/env.new" >"$env_file" || ! mkdir "$secrets/offsite" ||
		! printf "key\n" >"$secrets/offsite/password" || ! chmod 600 "$secrets/offsite/password"; then
		fail "cannot prepare the second run"
	fi
	env_sum=$(cksum <"$env_file")
	secrets_list=$(cd "$secrets" && find . -exec ls -ld {} + | awk '{ print $1, $NF }' | sort)
	secret_sum=$(cksum <"$secrets/offsite/password")
	inst sh "$INSTALL" --yes --storage s3 --owner-handle alex --owner-name Alex
	expect_rc 0
	[ "$(cksum <"$env_file")" = "$env_sum" ] || fail ".env changed"
	[ "$(mode_of "$env_file")" = "-rw-------" ] || fail ".env mode $(mode_of "$env_file")"
	[ "$(cd "$secrets" && find . -exec ls -ld {} + | awk '{ print $1, $NF }' | sort)" = "$secrets_list" ] || fail "secrets/ changed"
	[ "$(cksum <"$secrets/offsite/password")" = "$secret_sum" ] || fail "a secret changed"
	expect_env BOTHY_TRUSTED_PROXIES 198.51.100.7/32
	expect_in "$ERR" "existing"
	expect_in "$ERR" "owner already set up; skipped"
	expect_not_in "$OUT" "Enrollment code"
	expect_in "$OUT" "$PUBLIC_URL"
}

test_second_run_sets_gateway_when_default() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	env_file=$(std_env)
	if ! sed 's|^BOTHY_TRUSTED_PROXIES=.*|BOTHY_TRUSTED_PROXIES=127.0.0.1/32,::1/128|' "$env_file" >"$CASE/env.new" || ! cat "$CASE/env.new" >"$env_file"; then
		fail "cannot reset the proxies"
	fi
	rest=$(unchanged_lines "$env_file" | cksum)
	inst STUB_GW4=192.0.2.9 sh "$INSTALL" --yes --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_env BOTHY_TRUSTED_PROXIES 192.0.2.9/32
	[ "$(unchanged_lines "$env_file" | cksum)" = "$rest" ] || fail "other lines of .env changed"
	[ "$(mode_of "$env_file")" = "-rw-------" ] || fail ".env mode $(mode_of "$env_file")"
}

test_owner_exists_skips() {
	inst STUB_OWNER_EXISTS=1 sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_in "$ERR" "owner already set up; skipped"
	expect_not_in "$OUT" "Enrollment code"
}

test_bundle_key_signed_names_the_command() {
	inst STUB_DL_DIR="$WORK/dl-key" sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_not_in "$STUB_LOG" "cosign "
	expect_in "$ERR" "cosign verify-blob --key "
	expect_in "$ERR" "cosign-private-phase.pub"
}

test_other_version_in_the_folder_is_no_upgrade() {
	inst sh "$INSTALL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	printf 'version=0.9.9\n' >"$CASE/bothy/.bothy-install" || fail "cannot change the marker"
	env_sum=$(cksum <"$(std_env)")
	inst sh "$INSTALL" --yes --owner-handle alex --owner-name Alex
	expect_rc 1
	expect_in "$ERR" "install.sh: preflight: "
	expect_in "$ERR" "does not upgrade"
	[ "$(cksum <"$(std_env)")" = "$env_sum" ] || fail ".env changed"
}

test_trial_address_installs() {
	inst sh "$INSTALL_TRIAL" --yes --public-url "$PUBLIC_URL" --owner-handle alex --owner-name Alex
	expect_rc 0
	expect_in "$STUB_LOG" "url $TRIAL_BASE_URL$TEST_BUNDLE_NAME"
	expect_in "$ERR" "not a GitHub release"
	expect_in "$OUT" "Enrollment code (shown once): "
}
