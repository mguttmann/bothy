// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func schema(t *testing.T) *config.Schema {
	t.Helper()
	s, err := wire.ConfigSchema()
	if err != nil {
		t.Fatalf("ConfigSchema: %v", err)
	}
	return s
}

func TestConfigSchemaBuilds(t *testing.T) {
	t.Parallel()
	got := map[string]config.Key{}
	for _, k := range schema(t).Keys() {
		got[k.Name] = k
	}
	want := map[string]config.Key{
		"server.role":                          {Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"}, Chapter: "16-betrieb.md"},
		"server.listen_addr":                   {Kind: config.KindListenAddr, Default: "127.0.0.1:8080", Chapter: "16-betrieb.md"},
		"server.internal_listen_addr":          {Kind: config.KindListenAddr, Default: "127.0.0.1:8081", Chapter: "16-betrieb.md"},
		"server.tls_cert_ref":                  {Kind: config.KindSecretRef, Default: "", Chapter: "16-betrieb.md"},
		"server.tls_key_ref":                   {Kind: config.KindSecretRef, Default: "", Chapter: "16-betrieb.md"},
		"http.hsts_max_age":                    {Kind: config.KindDuration, Default: "31536000s", Min: "0s", Chapter: "19-sicherheit.md"},
		"http.max_json_body_bytes":             {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "02-architektur.md"},
		"http.read_header_timeout":             {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "02-architektur.md"},
		"http.idle_timeout":                    {Kind: config.KindDuration, Default: "120s", Min: "1ms", Chapter: "02-architektur.md"},
		"http.max_header_bytes":                {Kind: config.KindInt, Default: "65536", Min: "4096", Max: "16777216", Chapter: "02-architektur.md"},
		"http.max_json_depth":                  {Kind: config.KindInt, Default: "32", Min: "1", Max: "1000", Chapter: "02-architektur.md"},
		"http.request_timeout":                 {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "02-architektur.md"},
		"http.transfer_timeout":                {Kind: config.KindDuration, Default: "10m", Min: "1ms", Chapter: "02-architektur.md"},
		"http.unavailable_retry_after":         {Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h", Chapter: "02-architektur.md"},
		"file.max_upload_bytes":                {Kind: config.KindInt, Default: "104857600", Min: "1", Chapter: "03-datenmodell.md"},
		"file.storage.driver":                  {Kind: config.KindEnum, Default: "fs", Enum: []string{"fs", "s3"}, Chapter: "12-dateien.md"},
		"file.fs.root":                         {Kind: config.KindString, Default: "/var/lib/bothy/files", Chapter: "12-dateien.md"},
		"file.s3.endpoint":                     {Kind: config.KindString, Default: "", Chapter: "12-dateien.md"},
		"file.s3.region":                       {Kind: config.KindString, Default: "us-east-1", Chapter: "12-dateien.md"},
		"file.s3.bucket":                       {Kind: config.KindString, Default: "", Chapter: "12-dateien.md"},
		"file.s3.prefix":                       {Kind: config.KindString, Default: "", Chapter: "12-dateien.md"},
		"file.s3.use_path_style":               {Kind: config.KindBool, Default: "true", Chapter: "12-dateien.md"},
		"file.s3.checksum_mode":                {Kind: config.KindEnum, Default: "when_required", Enum: []string{"when_required", "when_supported"}, Chapter: "12-dateien.md"},
		"file.fs.min_free_bytes":               {Kind: config.KindInt, Default: "2147483648", Min: "0", Chapter: "12-dateien.md"},
		"file.upload_part_bytes":               {Kind: config.KindInt, Default: "8388608", Min: "1", Chapter: "12-dateien.md"},
		"file.upload_session_ttl":              {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "12-dateien.md"},
		"file.upload_max_open_sessions":        {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "12-dateien.md"},
		"file.quota_bytes_per_principal":       {Kind: config.KindInt, Default: "0", Min: "0", Chapter: "12-dateien.md"},
		"file.name_max_bytes":                  {Kind: config.KindInt, Default: "255", Min: "4", Chapter: "12-dateien.md"},
		"file.denied_mime_types":               {Kind: config.KindStringList, Default: "", Chapter: "12-dateien.md"},
		"file.s3.presigned_upload":             {Kind: config.KindBool, Default: "false", Chapter: "12-dateien.md"},
		"file.scan.mode":                       {Kind: config.KindEnum, Default: "required", Enum: []string{"required", "off"}, Chapter: "12-dateien.md"},
		"file.scan.clamd_address":              {Kind: config.KindString, Default: "unix:///run/clamav/clamd.sock", Chapter: "12-dateien.md"},
		"file.scan.chunk_bytes":                {Kind: config.KindInt, Default: "65536", Min: "1", Max: "2147483647", Chapter: "12-dateien.md"},
		"file.scan.timeout":                    {Kind: config.KindDuration, Default: "120s", Min: "1s", Chapter: "12-dateien.md"},
		"file.scan.max_concurrent":             {Kind: config.KindInt, Default: "4", Min: "1", Chapter: "12-dateien.md"},
		"file.scan.retry_backoff_cap":          {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "12-dateien.md"},
		"file.scan.health_interval":            {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "12-dateien.md"},
		"file.scan.clamd_stream_max_bytes":     {Kind: config.KindInt, Default: "104857600", Min: "1", Chapter: "12-dateien.md"},
		"file.sniff_bytes":                     {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "12-dateien.md"},
		"file.s3.access_key_ref":               {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_access_key", Chapter: "12-dateien.md"},
		"file.s3.secret_key_ref":               {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_secret_key", Chapter: "12-dateien.md"},
		"api.cors_allowed_origins":             {Kind: config.KindStringList, Default: "", Chapter: "06-api.md"},
		"server.public_url":                    {Kind: config.KindURL, Required: true, Chapter: "16-betrieb.md"},
		"server.trusted_proxies":               {Kind: config.KindCIDRList, Default: "127.0.0.1/32, ::1/128", Chapter: "16-betrieb.md"},
		"shutdown.drain_delay":                 {Kind: config.KindDuration, Default: "5s", Min: "0s", Max: "1h", Chapter: "02-architektur.md"},
		"shutdown.http_grace":                  {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architektur.md"},
		"shutdown.telemetry_flush_timeout":     {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "1h", Chapter: "02-architektur.md"},
		"shutdown.total_timeout":               {Kind: config.KindDuration, Default: "60s", Min: "1s", Max: "1h", Chapter: "02-architektur.md"},
		"shutdown.jobs_soft_timeout":           {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architektur.md"},
		"jobs.schema":                          {Kind: config.KindString, Default: "jobs", Chapter: "02-architektur.md"},
		"jobs.queue.default.max_workers":       {Kind: config.KindInt, Default: "8", Min: "1", Max: "10000", Chapter: "02-architektur.md"},
		"jobs.queue.files.max_workers":         {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architektur.md"},
		"jobs.queue.delivery.max_workers":      {Kind: config.KindInt, Default: "16", Min: "1", Max: "10000", Chapter: "02-architektur.md"},
		"jobs.queue.maintenance.max_workers":   {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architektur.md"},
		"jobs.queue.privacy.max_workers":       {Kind: config.KindInt, Default: "1", Min: "1", Max: "10000", Chapter: "02-architektur.md"},
		"message.max_content_bytes":            {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "03-datenmodell.md"},
		"principal.handle_max_chars":           {Kind: config.KindInt, Default: "32", Min: "1", Chapter: "03-datenmodell.md"},
		"principal.display_name_max_chars":     {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-datenmodell.md"},
		"auth.dpop_nonce_ttl":                  {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.session_max_age":                 {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "03-datenmodell.md"},
		"auth.refresh_token_ttl":               {Kind: config.KindDuration, Default: "14d", Min: "1m", Chapter: "03-datenmodell.md"},
		"auth.console_session_max_age":         {Kind: config.KindDuration, Default: "12h", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.console_session_idle_timeout":    {Kind: config.KindDuration, Default: "30m", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.refresh_reuse_grace":             {Kind: config.KindDuration, Default: "30s", Min: "0s", Chapter: "03-datenmodell.md"},
		"identity.last_active_granularity":     {Kind: config.KindDuration, Default: "5m", Min: "0s", Chapter: "03-datenmodell.md"},
		"auth.step_up_challenge_ttl":           {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.step_up_fail_limit":              {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "08-identitaet.md"},
		"auth.login_fail_window":               {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.cleanup_interval":                {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.device_pending_ttl":              {Kind: config.KindDuration, Default: "24h", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.device_verification_code_digits": {Kind: config.KindInt, Default: "6", Min: "6", Max: "12", Chapter: "08-identitaet.md"},
		"auth.pairing_code_ttl":                {Kind: config.KindDuration, Default: "5m", Min: "30s", Chapter: "08-identitaet.md"},
		"auth.pairing_code_chars":              {Kind: config.KindInt, Default: "8", Min: "8", Max: "16", Chapter: "08-identitaet.md"},
		"auth.enrollment_code_ttl":             {Kind: config.KindDuration, Default: "72h", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.enrollment_code_max_attempts":    {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "08-identitaet.md"},
		"auth.dpop_proof_max_age":              {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.clock_leeway":                    {Kind: config.KindDuration, Default: "30s", Min: "0s", Chapter: "08-identitaet.md"},
		"auth.token_pepper_ref":                {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper", Chapter: "08-identitaet.md"},
		"auth.dpop_required_for":               {Kind: config.KindStringList, Default: `["agent"]`, Chapter: "08-identitaet.md"},
		"auth.api_token_max_ttl":               {Kind: config.KindDuration, Default: "365d", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.personal_token_max_ttl":          {Kind: config.KindDuration, Default: "90d", Min: "1s", Chapter: "08-identitaet.md"},
		"api.page_size_default":                {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "06-api.md"},
		"api.page_size_max":                    {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "06-api.md"},
		"authz.snapshot_cache_max_entries":     {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "07-rechte.md"},
		"authz.expiry_sweep_interval":          {Kind: config.KindDuration, Default: "1m", Min: "1s", Chapter: "07-rechte.md"},
		"authz.expiry_reminder_lead":           {Kind: config.KindDuration, Default: "48h", Min: "1s", Chapter: "07-rechte.md"},
		"authz.expiry_sweep_batch_max":         {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "07-rechte.md"},
		"authz.view_as_max_duration":           {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "07-rechte.md"},
		"auth.step_up_max_age":                 {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "03-datenmodell.md"},
		"group.name_max_chars":                 {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"group.description_max_chars":          {Kind: config.KindInt, Default: "500", Min: "1", Chapter: "03-datenmodell.md"},
		"space.name_max_chars":                 {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"channel.name_max_chars":               {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"channel.topic_max_chars":              {Kind: config.KindInt, Default: "1024", Min: "1", Chapter: "03-datenmodell.md"},
		"forum_tag.name_max_chars":             {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-datenmodell.md"},
		"thread.max_tags":                      {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "03-datenmodell.md"},
		"thread.placeholder_titles": {Kind: config.KindStringList, Default: `["Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]`,
			Chapter: "09-agenten.md"},
		"channel.history_backfill_limit_default": {Kind: config.KindInt, Default: "20", Min: "0", Chapter: "03-datenmodell.md"},
		"channel.history_backfill_limit_max":     {Kind: config.KindInt, Default: "200", Min: "0", Chapter: "03-datenmodell.md"},
		"channel.prompt_max_bytes":               {Kind: config.KindInt, Default: "32768", Min: "1", Chapter: "03-datenmodell.md"},
		"host.name_max_chars":                    {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"host.os_user_max_chars":                 {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-datenmodell.md"},
		"terminal.pairing_code_ttl":              {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.session_token_ttl":             {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.fingerprint_confirm_chars":     {Kind: config.KindInt, Default: "8", Min: "1", Chapter: "24-terminal.md"},
		"terminal.fingerprint_max_attempts":      {Kind: config.KindInt, Default: "3", Min: "1", Chapter: "24-terminal.md"},
		"terminal.report_label_max_chars":        {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "24-terminal.md"},
		"terminal.report_max_entries":            {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "24-terminal.md"},
		"terminal.step_up_max_age":               {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.quick_reattach_window":         {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "24-terminal.md"},
		"terminal.host_heartbeat_interval":       {Kind: config.KindDuration, Default: "20s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.host_heartbeat_miss_limit":     {Kind: config.KindInt, Default: "3", Min: "1", Chapter: "24-terminal.md"},
		"terminal.host_auth_timeout":             {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.host_duplicate_probe_timeout":  {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.open_timeout":                  {Kind: config.KindDuration, Default: "20s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.relay_buffer_max_bytes":        {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "24-terminal.md"},
		"audit.reason_max_chars":                 {Kind: config.KindInt, Default: "500", Min: "1", Chapter: "19-sicherheit.md"},
		"egress.allowed_hosts":                   {Kind: config.KindStringList, Default: "", Chapter: "19-sicherheit.md"},
		"egress.timeout":                         {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "19-sicherheit.md"},
		"egress.max_response_bytes":              {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "19-sicherheit.md"},
		"secrets.dir":                            {Kind: config.KindString, Default: "/run/bothy/secrets", Chapter: "16-betrieb.md"},
		"secrets.cache_ttl":                      {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "16-betrieb.md"},
		"secrets.signing_key_dir":                {Kind: config.KindString, Default: "/var/lib/bothy/signing-keys", Chapter: "16-betrieb.md"},
		"console.assets_dir":                     {Kind: config.KindString, Default: "", Chapter: "16-betrieb.md"},

		"signing_key.cache_ttl":                      {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "19-sicherheit.md"},
		"signing_key.rotation_check_interval":        {Kind: config.KindDuration, Default: "1m", Min: "1s", Chapter: "19-sicherheit.md"},
		"signing_key.access_token_rotation_interval": {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "19-sicherheit.md"},
		"signing_key.invocation_rotation_interval":   {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "09-agenten.md"},
		"agent.jwks_max_age":                         {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "09-agenten.md"},
		"agent.invocation_token_ttl":                 {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "04-sync-protokoll.md"},
		"auth.access_token_ttl":                      {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "08-identitaet.md"},
		"webhook.delivery_timeout":                   {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "11-erweiterungen.md"},

		"database.host":                         {Kind: config.KindString, Default: "localhost", Chapter: "16-betrieb.md"},
		"database.port":                         {Kind: config.KindInt, Default: "5432", Min: "1", Max: "65535", Chapter: "16-betrieb.md"},
		"database.name":                         {Kind: config.KindString, Default: "bothy", Chapter: "16-betrieb.md"},
		"database.user":                         {Kind: config.KindString, Default: "bothy_app", Chapter: "16-betrieb.md"},
		"database.password_ref":                 {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-betrieb.md"},
		"database.sslmode":                      {Kind: config.KindEnum, Default: "verify-full", Enum: []string{"disable", "verify-full"}, Chapter: "16-betrieb.md"},
		"database.sslrootcert":                  {Kind: config.KindString, Default: "", Chapter: "16-betrieb.md"},
		"database.max_conns":                    {Kind: config.KindInt, Default: "20", Min: "1", Max: "2147483647", Chapter: "16-betrieb.md"},
		"database.acquire_timeout":              {Kind: config.KindDuration, Default: "2s", Min: "1ms", Chapter: "02-architektur.md"},
		"database.statement_timeout":            {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "02-architektur.md"},
		"database.lock_timeout":                 {Kind: config.KindDuration, Default: "2s", Min: "1ms", Max: "24d", Chapter: "02-architektur.md"},
		"database.idle_in_tx_timeout":           {Kind: config.KindDuration, Default: "10s", Min: "1ms", Max: "24d", Chapter: "02-architektur.md"},
		"database.tx_retry_max":                 {Kind: config.KindInt, Default: "2", Min: "0", Chapter: "02-architektur.md"},
		"database.listen_reconnect_backoff_cap": {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "02-architektur.md"},
		"startup.dependency_wait":               {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "02-architektur.md"},
		"migrate.lock_timeout":                  {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "16-betrieb.md"},
		"migrate.bootstrap_user":                {Kind: config.KindString, Default: "postgres", Chapter: "16-betrieb.md"},
		"migrate.bootstrap_password_ref":        {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_superuser_password", Chapter: "16-betrieb.md"},
		"migrate.owner_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_owner_password", Chapter: "16-betrieb.md"},
		"migrate.app_password_ref":              {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-betrieb.md"},
		"migrate.maint_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_maint_password", Chapter: "16-betrieb.md"},
		"migrate.monitor_password_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_monitor_password", Chapter: "16-betrieb.md"},

		"instance.environment":                 {Kind: config.KindEnum, Default: "prod", Enum: []string{"dev", "staging", "prod"}, Chapter: "16-betrieb.md"},
		"telemetry.enabled":                    {Kind: config.KindBool, Default: "true", Chapter: "17-observability.md"},
		"telemetry.traces.sampler_ratio":       {Kind: config.KindFloat, Default: "1.0", Min: "0", Max: "1", Chapter: "17-observability.md"},
		"telemetry.traces.trust_remote_parent": {Kind: config.KindBool, Default: "false", Chapter: "17-observability.md"},
		"telemetry.logs.exporter":              {Kind: config.KindEnum, Default: "both", Enum: []string{"otlp", "stdout", "both"}, Chapter: "17-observability.md"},
		"telemetry.logs.level":                 {Kind: config.KindEnum, Default: "info", Enum: []string{"debug", "info", "warn", "error"}, Chapter: "17-observability.md"},
		"telemetry.pseudonym_key_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/telemetry_pseudonym_key", Chapter: "17-observability.md"},
		"privacy.telemetry_pseudonym_rotation": {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-datenschutz.md"},

		"audit.deny_aggregate_window":        {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "05-aktionen.md"},
		"audit.deny_aggregate_max_keys":      {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "05-aktionen.md"},
		"audit.checkpoint_interval":          {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_sink":              {Kind: config.KindString, Default: "", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_s3_endpoint":       {Kind: config.KindString, Default: "", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_s3_region":         {Kind: config.KindString, Default: "us-east-1", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_s3_use_path_style": {Kind: config.KindBool, Default: "true", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_s3_access_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_checkpoint_s3_access_key", Chapter: "19-sicherheit.md"},
		"audit.checkpoint_s3_secret_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_checkpoint_s3_secret_key", Chapter: "19-sicherheit.md"},
		"retention.audit":                    {Kind: config.KindDuration, Default: "365d", Min: "1d", Chapter: "20-datenschutz.md"},
		"privacy.audit_ip_key_period":        {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-datenschutz.md"},
		"privacy.audit_ip_key_retention":     {Kind: config.KindDuration, Default: "90d", Min: "1s", Chapter: "20-datenschutz.md"},
		"privacy.audit_ip_kek_ref":           {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_ip_kek", Chapter: "20-datenschutz.md"},
		"ratelimit.max_tracked_keys":         {Kind: config.KindInt, Default: "100000", Min: "1", Max: "100000000", Chapter: "02-architektur.md"},
		"ratelimit.sweep_interval":           {Kind: config.KindDuration, Default: "1m", Min: "1s", Max: "1h", Chapter: "02-architektur.md"},

		"sync.event_log_partition_size":       {Kind: config.KindInt, Default: "1000000", Min: "1", Max: "1000000000000", Chapter: "03-datenmodell.md"},
		"sync.event_retention":                {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "04-sync-protokoll.md"},
		"sync.event_payload_max_bytes":        {Kind: config.KindInt, Default: "65536", Min: "3", Max: "1048576", Chapter: "04-sync-protokoll.md"},
		"sync.log_poll_interval":              {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "02-architektur.md"},
		"sync.tail_batch_size":                {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architektur.md"},
		"sync.partition_maintenance_interval": {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "02-architektur.md"},
		"sync.idempotency_key_ttl":            {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "04-sync-protokoll.md"},
		"sync.idempotency_cleanup_interval":   {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "02-architektur.md"},
		"sync.idempotency_cleanup_batch_size": {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architektur.md"},
		"sync.event_log_partitions_ahead":     {Kind: config.KindInt, Default: "3", Min: "1", Max: "1000", Chapter: "02-architektur.md"},
		"sync.catchup_page_size":              {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "04-sync-protokoll.md"},
		"sync.catchup_max_events":             {Kind: config.KindInt, Default: "50000", Min: "1", Max: "1000000000000", Chapter: "04-sync-protokoll.md"},
		"sync.cursor_advance_every":           {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "04-sync-protokoll.md"},

		"consumer.batch_size":          {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "02-architektur.md"},
		"consumer.poll_interval":       {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "02-architektur.md"},
		"consumer.pending_alert_after": {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "02-architektur.md"},

		"branding.product_name_max_chars": {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-datenmodell.md"},
		"message.max_attachments":         {Kind: config.KindInt, Default: "10", Min: "0", Chapter: "03-datenmodell.md"},
		// M1-56: the mention limit of sending.
		"message.max_mentions": {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-datenmodell.md"},
		// M1-57: the content limit of the history lists.
		"message.list_content_max_bytes":     {Kind: config.KindInt, Default: "16384", Min: "1", Chapter: "06-api.md"},
		"thread.title_max_chars":             {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "03-datenmodell.md"},
		"template.name_max_chars":            {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"template.max_per_scope":             {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-datenmodell.md"},
		"reaction.emoji_max_bytes":           {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-datenmodell.md"},
		"reaction.max_distinct_per_message":  {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "03-datenmodell.md"},
		"pin.max_per_thread":                 {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-datenmodell.md"},
		"voice.max_duration":                 {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "12-dateien.md"},
		"auth.local_password_enabled":        {Kind: config.KindBool, Default: "true", Chapter: "08-identitaet.md"},
		"auth.login_flow_ttl":                {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.webauthn_rp_id":                {Kind: config.KindString, Default: "", Chapter: "08-identitaet.md"},
		"auth.webauthn_origins":              {Kind: config.KindStringList, Default: "", Chapter: "08-identitaet.md"},
		"auth.server_code_ttl":               {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.oidc_jwks_cache_ttl":           {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.oidc_client_assertion_ttl":     {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.device_inactive_revoke_after":  {Kind: config.KindDuration, Default: "180d", Min: "1h", Chapter: "08-identitaet.md"},
		"auth.trusted_header_bind_requests":  {Kind: config.KindBool, Default: "true", Chapter: "08-identitaet.md"},
		"auth.idp_recheck_interval":          {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.group_claim_max_age":           {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "08-identitaet.md"},
		"auth.argon2id_memory_kib":           {Kind: config.KindInt, Default: "65536", Min: "19456", Max: "4194304", Chapter: "08-identitaet.md"},
		"auth.argon2id_iterations":           {Kind: config.KindInt, Default: "3", Min: "2", Max: "1000", Chapter: "08-identitaet.md"},
		"auth.argon2id_parallelism":          {Kind: config.KindInt, Default: "1", Min: "1", Max: "255", Chapter: "08-identitaet.md"},
		"auth.password_min_length":           {Kind: config.KindInt, Default: "12", Min: "1", Max: "1024", Chapter: "08-identitaet.md"},
		"auth.login_fail_limit":              {Kind: config.KindInt, Default: "10", Min: "1", Chapter: "08-identitaet.md"},
		"auth.login_lock_duration":           {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "08-identitaet.md"},
		"auth.totp_key_ref":                  {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/totp_key", Chapter: "08-identitaet.md"},
		"client.min_version.ios":             {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.android":         {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.macos":           {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.windows":         {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.ios":     {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.android": {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.macos":   {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.windows": {Kind: config.KindString, Default: "", Chapter: "18-release.md"},

		// M2-24
		"app.manifest_max_bytes":        {Kind: config.KindInt, Default: "65536", Min: "1", Chapter: "11-erweiterungen.md"},
		"webhook.allowed_hosts":         {Kind: config.KindStringList, Default: "", Chapter: "03-datenmodell.md"},
		"command.max_per_principal":     {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "09-agenten.md"},
		"command.description_max_chars": {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "09-agenten.md"},

		// M3-13
		"search.languages":  {Kind: config.KindStringList, Default: `["german","english"]`, Chapter: "14-suche.md"},
		"search.locale_map": {Kind: config.KindStringMap, Default: `{"de":"german","en":"english"}`, Chapter: "14-suche.md"},
		// M2-26
		"webhook.max_consecutive_failures":     {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "03-datenmodell.md"},
		"webhook.retry_backoff_base":           {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "03-datenmodell.md"},
		"webhook.retry_backoff_cap":            {Kind: config.KindDuration, Default: "1h", Min: "1ms", Chapter: "03-datenmodell.md"},
		"webhook.max_pending_per_subscription": {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "11-erweiterungen.md"},
		"webhook.retry_max_age":                {Kind: config.KindDuration, Default: "72h", Min: "1ms", Chapter: "11-erweiterungen.md"},
		"webhook.secret_rotation_overlap":      {Kind: config.KindDuration, Default: "24h", Min: "1ms", Chapter: "11-erweiterungen.md"},
		"webhook.hmac_master_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/webhook_hmac_master_key",
			Chapter: "11-erweiterungen.md"},
		// M3-18
		"notify.user_default_level": {Kind: config.KindEnum, Default: "mentions", Enum: []string{"all", "mentions", "mute"}, Chapter: "13-push-benachrichtigungen.md"},
		// The agent consumers (M2-07, 04 "Konfigurationswerte dieses Kapitels").
		"agent.ack_timeout":          {Kind: config.KindDuration, Default: "30s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"agent.ack_persist_interval": {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"agent.max_inflight_events":  {Kind: config.KindInt, Default: "100", Min: "1", Max: "2147483647", Chapter: "04-sync-protokoll.md"},
		// M1-64
		"sync.ws_max_frame_bytes":   {Kind: config.KindInt, Default: "1048576", Min: "1024", Max: "16777216", Chapter: "04-sync-protokoll.md"},
		"sync.identify_timeout":     {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"sync.heartbeat_interval":   {Kind: config.KindDuration, Default: "30s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"sync.heartbeat_miss_limit": {Kind: config.KindInt, Default: "2", Min: "0", Max: "100", Chapter: "04-sync-protokoll.md"},
		"sync.resume_window":        {Kind: config.KindDuration, Default: "5m", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"sync.reauth_lead_time":     {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"sync.outbound_queue_max":   {Kind: config.KindInt, Default: "10000", Min: "1", Max: "1000000", Chapter: "04-sync-protokoll.md"},
		"gateway.write_timeout":     {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "02-architektur.md"},
		// M1-66
		"sync.max_focused_threads": {Kind: config.KindInt, Default: "20", Min: "1", Max: "1000", Chapter: "04-sync-protokoll.md"},
		"sync.typing_min_interval": {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
		"sync.typing_ttl":          {Kind: config.KindDuration, Default: "8s", Min: "1ms", Chapter: "04-sync-protokoll.md"},
	}
	// Two keys per class of 05 "Rate-Limit-Klassen" (M1-19), with the defaults of 05 section 10.
	for class, def := range map[string][2]string{
		"rl.public_auth": {"30", "10"}, "rl.gateway_connect": {"12", "4"}, "rl.read": {"600", "100"},
		"rl.write": {"120", "30"}, "rl.message": {"60", "20"}, "rl.trigger": {"30", "10"},
		"rl.interaction": {"60", "20"}, "rl.upload": {"30", "10"}, "rl.search": {"60", "20"},
		"rl.typing": {"30", "5"}, "rl.admin": {"60", "20"}, "rl.export": {"2", "2"},
		"rl.agent_runtime": {"1200", "200"}, "rl.webhook_in": {"60", "20"}, "rl.import": {"120", "30"},
		"rl.terminal": {"20", "5"}, "rl.host": {"120", "30"},
	} {
		want["ratelimit."+class+".per_minute"] = config.Key{Kind: config.KindInt, Default: def[0], Min: "1", Max: "1000000", Chapter: "05-aktionen.md"}
		want["ratelimit."+class+".burst"] = config.Key{Kind: config.KindInt, Default: def[1], Min: "1", Max: "1000000", Chapter: "05-aktionen.md"}
	}
	if len(got) != len(want) {
		t.Errorf("schema has %d keys, want %d", len(got), len(want))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("key %s missing", name)
			continue
		}
		if g.Doc == "" || !strings.HasSuffix(g.Doc, ".") {
			t.Errorf("%s: Doc %q is not one sentence", name, g.Doc)
		}
		g.Name, g.Doc = "", ""
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s = %+v, want %+v", name, g, w)
		}
	}
}

// TestConfigKeysMatchChapterTables holds fixed requirement 9 of the brief: every key in the
// schema has a row in the table "Konfigurationswerte dieses Kapitels" of its chapter with the
// same default (M1-08 later checks all keys of all chapters).
func TestConfigKeysMatchChapterTables(t *testing.T) {
	t.Parallel()
	for _, k := range schema(t).Keys() {
		cell, ok := chapterDefault(t, k.Chapter, k.Name)
		if !ok {
			t.Errorf("%s: no row in the table of %s", k.Name, k.Chapter)
			continue
		}
		want := strings.ReplaceAll(cell, "`", "")
		switch cell {
		case "leer":
			want = ""
		case "keiner, Pflicht":
			want = ""
			if !k.Required {
				t.Errorf("%s: chapter says required, schema does not", k.Name)
			}
		}
		if k.Default != want {
			t.Errorf("%s: schema default %q, chapter %s says %q", k.Name, k.Default, k.Chapter, cell)
		}
	}
}

// chapterDefault returns the cell of the column "Standard" in the row of name in the section
// "Konfigurationswerte dieses Kapitels" of chapter.
func chapterDefault(t *testing.T, chapter, name string) (string, bool) {
	t.Helper()
	text := readSpec(t, chapter)
	start := strings.Index(text, "Konfigurationswerte dieses Kapitels\n")
	if start < 0 {
		t.Fatalf("%s has no section Konfigurationswerte dieses Kapitels", chapter)
	}
	column := -1
	for _, line := range strings.Split(text[start:], "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if column < 0 {
			for i, c := range cells {
				if c == "Standard" {
					column = i
				}
			}
			continue
		}
		if cells[0] == "`"+name+"`" && column < len(cells) {
			return cells[column], true
		}
		// Shorthand rows of 05: "`ratelimit.rl.read.per_minute` / `.burst`" with "`600` / `100`".
		if names := strings.Split(cells[0], " / "); len(names) > 1 && column < len(cells) {
			values := strings.Split(cells[column], " / ")
			first := strings.Trim(names[0], "`")
			base := first[:strings.LastIndex(first, ".")]
			for i, n := range names {
				n = strings.Trim(n, "`")
				if i > 0 {
					n = base + n
				}
				if n == name && len(values) == len(names) {
					return values[i], true
				}
			}
		}
	}
	return "", false
}

func readSpec(t *testing.T, chapter string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && info.IsDir() {
			root, err := os.OpenRoot(filepath.Join(dir, "docs", "spec"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data, err := root.ReadFile(chapter)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

func TestDefaultsPassCrossChecks(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatalf("Load with defaults only: %v", err)
	}
	if v.String("server.role") != "all" {
		t.Errorf("server.role = %q", v.String("server.role"))
	}
}

// load returns the problems of loading the server schema with env (nil when it loads).
func load(t *testing.T, env ...string) []config.Problem {
	t.Helper()
	if !slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "BOTHY_SERVER__PUBLIC_URL=") }) {
		env = append(testEnv(), env...)
	}
	_, err := config.Load(schema(t), config.Sources{Env: env})
	if err == nil {
		return nil
	}
	var le *config.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("Load error %T", err)
	}
	return le.Problems
}

func TestListenersDistinct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		public, internal string
		conflict         bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{":8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", ":8080", true},
		{"[::]:8080", "127.0.0.1:8080", true},
		{"0.0.0.0:8080", "[::1]:8080", true},
		{"[::ffff:127.0.0.1]:8080", "127.0.0.1:8080", true},
		{"[::1]:8080", "[::1]:8080", true},
		{"[::%eth0]:8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "[::%1]:8080", true},
		{"[fe80::1%eth0]:8080", "[fe80::1%eth0]:8080", true},
		{"[fe80::1%eth0]:8080", "[fe80::1%eth1]:8080", false},
		{"127.0.0.1:0", "127.0.0.1:0", false},
		{":0", ":0", false},
		{"127.0.0.1:8080", "127.0.0.1:8081", false},
		{"127.0.0.1:8080", "127.0.0.2:8080", false},
		{"127.0.0.1:8080", "[::1]:8080", false},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_SERVER__LISTEN_ADDR="+c.public, "BOTHY_SERVER__INTERNAL_LISTEN_ADDR="+c.internal)
		if !c.conflict {
			if got != nil {
				t.Errorf("%s and %s: problems %v, want none", c.public, c.internal, got)
			}
			continue
		}
		want := "config: server.listen_addr, server.internal_listen_addr: listeners_distinct: " +
			"the public and the internal listener must not share a port on the same or an unspecified address; only port 0 may repeat " +
			"(source: server.listen_addr from env BOTHY_SERVER__LISTEN_ADDR, server.internal_listen_addr from env BOTHY_SERVER__INTERNAL_LISTEN_ADDR)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("%s and %s: problems %v, want\n%s", c.public, c.internal, got, want)
		}
		noValues(t, got, c.public, c.internal)
	}
}

func TestJSONLimitAboveMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		json, message string
		ok            bool
	}{
		{"262144", "262144", false},
		{"262143", "262144", false},
		{"262145", "262144", true},
		{"1048576", "1048577", false},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_HTTP__MAX_JSON_BODY_BYTES="+c.json, "BOTHY_MESSAGE__MAX_CONTENT_BYTES="+c.message)
		if c.ok {
			if got != nil {
				t.Errorf("%s > %s: problems %v", c.json, c.message, got)
			}
			continue
		}
		want := "config: http.max_json_body_bytes, message.max_content_bytes: json_limit_above_message: " +
			"http.max_json_body_bytes must be greater than message.max_content_bytes " +
			"(source: http.max_json_body_bytes from env BOTHY_HTTP__MAX_JSON_BODY_BYTES, message.max_content_bytes from env BOTHY_MESSAGE__MAX_CONTENT_BYTES)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("%s vs %s: problems %v, want\n%s", c.json, c.message, got, want)
		}
		noValues(t, got, c.json, c.message)
	}
}

// TestSecretsDirCleanAbsolute holds that bothy config check rejects every secrets.dir the secret
// provider rejects (absolute and clean like a file: reference, not the root, without NUL and
// "<"): the loader checks the key itself (docs/DECISIONS.md M1-09/15), so config check reports
// what the start would.
func TestSecretsDirCleanAbsolute(t *testing.T) {
	t.Parallel()
	const (
		notAbsolute = "must be an absolute path"
		hasNUL      = "must not contain a NUL character"
		isRoot      = "must not be the root directory"
		hasAngle    = "must not contain <"
		notClean    = "must be a clean path without . or .. elements and without repeated or trailing slashes"
	)
	cases := []struct {
		dir, want string
	}{
		{"/run/bothy/secrets", ""},
		{"/srv/bothy-secrets", ""},
		{"", notAbsolute},
		{"run/bothy/secrets", notAbsolute},
		{"./secrets", notAbsolute},
		{"/", isRoot},
		{"/run/bothy/\x00secrets", hasNUL},
		{"/run/<secrets.dir>", hasAngle},
		{"/run/bothy/secrets/", notClean},
		{"/run//bothy/secrets", notClean},
		{"/run/bothy/./secrets", notClean},
		{"/run/bothy/../secrets", notClean},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_SECRETS__DIR="+c.dir)
		if c.want == "" {
			if got != nil {
				t.Errorf("secrets.dir %q: problems %v, want none", c.dir, got)
			}
			continue
		}
		want := "config: secrets.dir: " + c.want + " (source: env BOTHY_SECRETS__DIR)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("secrets.dir %q: problems %v, want\n%s", c.dir, got, want)
		}
		if c.dir != "" && c.dir != "/" {
			noValues(t, got, c.dir)
		}
	}
}

// TestSecretsCacheTTLAtLeastOneSecond holds the lower bound of secrets.cache_ttl
// (docs/DECISIONS.md M1-09/24): with 0 every call would read the secret, and while a refresh
// fails every call would log a line and count a failure instead of one per TTL.
func TestSecretsCacheTTLAtLeastOneSecond(t *testing.T) {
	t.Parallel()
	if got := load(t, "BOTHY_SECRETS__CACHE_TTL=1s"); got != nil {
		t.Errorf("1s: problems %v, want none", got)
	}
	got := load(t, "BOTHY_SECRETS__CACHE_TTL=0s")
	if want := "config: secrets.cache_ttl: must be at least 1s (source: env BOTHY_SECRETS__CACHE_TTL)"; len(got) != 1 || got[0].String() != want {
		t.Errorf("0s: problems %v, want\n%s", got, want)
	}
}

func noValues(t *testing.T, problems []config.Problem, values ...string) {
	t.Helper()
	text := (&config.LoadError{Problems: problems}).Error()
	for _, v := range values {
		if strings.Contains(text, v) {
			t.Errorf("problem text echoes the value %q:\n%s", v, text)
		}
	}
}

// testPublicURL sets server.public_url, the one key without a default (16 "Konfigurationswerte").
const testPublicURL = "BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"

// testEnv is the smallest environment that loads: the defaults plus server.public_url.
func testEnv() []string { return []string{testPublicURL} }

// TestPublicURLAndCORSOriginsAreOrigins: server.public_url and every CORS entry must be an origin
// (the chain builds the DPoP htu from it and compares the Origin header exactly).
func TestPublicURLAndCORSOriginsAreOrigins(t *testing.T) {
	t.Parallel()
	if got := load(t); got != nil {
		t.Fatalf("defaults with public_url: %v", got)
	}
	cases := map[string]string{
		"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com/app":                          "server.public_url",
		"BOTHY_API__CORS_ALLOWED_ORIGINS=https://app.example.com,https://x.example.com/": "api.cors_allowed_origins",
	}
	for env, key := range cases {
		got := load(t, env)
		if len(got) != 1 || !strings.Contains(got[0].String(), key) {
			t.Errorf("%s: problems %v", env, got)
		}
	}
	if got := load(t, "BOTHY_API__CORS_ALLOWED_ORIGINS=https://app.example.com"); got != nil {
		t.Errorf("valid origin: %v", got)
	}
	_, err := config.Load(schema(t), config.Sources{})
	if err == nil || !strings.Contains(err.Error(), "server.public_url") {
		t.Errorf("without server.public_url: %v", err)
	}
}

// TestTLSKeysAndCrossChecks holds Scope 2 of brief M1-21: server.tls_cert_ref and
// server.tls_key_ref are optional secret references (env BOTHY_SERVER__TLS_CERT_REF and
// BOTHY_SERVER__TLS_KEY_REF), both set or both empty, and with TLS server.public_url uses https.
// The problems name keys and sources, never a value.
func TestTLSKeysAndCrossChecks(t *testing.T) {
	t.Parallel()
	const (
		cert = "BOTHY_SERVER__TLS_CERT_REF=file:/run/bothy/secrets/tls_cert"
		key  = "BOTHY_SERVER__TLS_KEY_REF=file:/run/bothy/secrets/tls_key"
	)
	if got := load(t); got != nil {
		t.Fatalf("no TLS: %v", got)
	}
	if got := load(t, cert, key); got != nil {
		t.Fatalf("TLS with an https public URL: %v", got)
	}
	if got := load(t, "BOTHY_SERVER__TLS_CERT_REF=", "BOTHY_SERVER__TLS_KEY_REF="); got != nil {
		t.Fatalf("both empty: %v", got)
	}
	pair := "config: server.tls_cert_ref, server.tls_key_ref: tls_pair: " +
		"server.tls_cert_ref and server.tls_key_ref must both be set or both be empty"
	for _, env := range []string{cert, key} {
		got := load(t, env)
		if len(got) != 1 || !strings.HasPrefix(got[0].String(), pair) {
			t.Errorf("%s alone: problems %v, want %s", env, got, pair)
		}
		noValues(t, got, "/run/bothy/secrets")
	}
	got := load(t, "BOTHY_SERVER__PUBLIC_URL=http://chat.example.com", cert, key)
	want := "config: server.public_url, server.tls_cert_ref, server.tls_key_ref: tls_https: " +
		"with server.tls_cert_ref and server.tls_key_ref set, server.public_url must use https"
	if len(got) != 1 || !strings.HasPrefix(got[0].String(), want) {
		t.Errorf("TLS with an http public URL: problems %v, want %s", got, want)
	}
	noValues(t, got, "chat.example.com", "/run/bothy/secrets")
}

// TestHSTSMaxAgeWholeSeconds holds http.hsts_max_age (19 "Konfigurationswerte", SEC-TLS-3): a
// duration in whole seconds, 0 or more; max-age has no fractions.
func TestHSTSMaxAgeWholeSeconds(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"0s", "1s", "31536000s", "2m"} {
		if got := load(t, "BOTHY_HTTP__HSTS_MAX_AGE="+ok); got != nil {
			t.Errorf("%s: problems %v, want none", ok, got)
		}
	}
	for _, bad := range []string{"1500ms", "-1s", "1ms"} {
		got := load(t, "BOTHY_HTTP__HSTS_MAX_AGE="+bad)
		if len(got) != 1 || !strings.Contains(got[0].String(), "http.hsts_max_age") {
			t.Errorf("%s: problems %v, want one about http.hsts_max_age", bad, got)
		}
		noValues(t, got, bad)
	}
}

// TestAuditIPKeyRetentionAtLeastPeriod holds the cross-check of M1-24: a key must outlive its own
// period, so privacy.audit_ip_key_retention below privacy.audit_ip_key_period is refused.
func TestAuditIPKeyRetentionAtLeastPeriod(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		period, retention string
		ok                bool
	}{
		{"30d", "90d", true},
		{"30d", "30d", true},
		{"30d", "29d", false},
	} {
		got := load(t, "BOTHY_PRIVACY__AUDIT_IP_KEY_PERIOD="+c.period, "BOTHY_PRIVACY__AUDIT_IP_KEY_RETENTION="+c.retention)
		if (len(got) == 0) != c.ok {
			t.Errorf("period %s retention %s: problems %v, want ok %v", c.period, c.retention, got, c.ok)
		}
		if len(got) > 0 && !strings.Contains(got[0].String(), "privacy.audit_ip_key_retention") {
			t.Errorf("the problem does not name the key: %v", got)
		}
	}
}

// TestConfigCheckConsumerAlertBelowRetention: consumer.pending_alert_after must be strictly
// below sync.event_retention (cross-check consumer_alert_below_retention, D-M2-01-6), so the
// alarm BothyEventConsumerLag fires before a stalled consumer loses events; the problem names
// both keys and no value.
func TestConfigCheckConsumerAlertBelowRetention(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		alert, retention string
		ok               bool
	}{
		{"15m", "30d", true},
		{"29d", "30d", true},
		{"30d", "30d", false},
		{"31d", "30d", false},
		{"2h", "1h", false},
	} {
		got := load(t, "BOTHY_CONSUMER__PENDING_ALERT_AFTER="+c.alert, "BOTHY_SYNC__EVENT_RETENTION="+c.retention)
		if (len(got) == 0) != c.ok {
			t.Errorf("alert %s retention %s: problems %v, want ok %v", c.alert, c.retention, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "consumer.pending_alert_after") || !strings.Contains(msg, "sync.event_retention") {
			t.Errorf("the problem does not name both keys: %q", msg)
		}
		for _, v := range []string{c.alert, c.retention} {
			if strings.Contains(msg, v) {
				t.Errorf("the problem names the value %s: %q", v, msg)
			}
		}
	}
}

// TestConfigCheckWebhookBackoffOrder: webhook.retry_backoff_base must be at most
// webhook.retry_backoff_cap (cross-check webhook_backoff_order, M2-26); the problem names both
// keys and no value.
func TestConfigCheckWebhookBackoffOrder(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		base, limit string
		ok          bool
	}{
		{"10s", "1h", true},
		{"1h", "1h", true},
		{"61m", "1h", false},
	} {
		got := load(t, "BOTHY_WEBHOOK__RETRY_BACKOFF_BASE="+c.base, "BOTHY_WEBHOOK__RETRY_BACKOFF_CAP="+c.limit)
		checkPairProblem(t, got, c.ok, "webhook.retry_backoff_base", "webhook.retry_backoff_cap", c.base, c.limit)
	}
}

// TestConfigCheckWebhookRetryWithinRetention: webhook.retry_max_age must be strictly below
// sync.event_retention (cross-check webhook_retry_within_retention, M2-26), because a retry needs
// its event in the log; equal and greater fail, smaller passes, and the problem names both keys
// and no value.
func TestConfigCheckWebhookRetryWithinRetention(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		age, retention string
		ok             bool
	}{
		{"72h", "30d", true},
		{"30d", "30d", false},
		{"31d", "30d", false},
	} {
		got := load(t, "BOTHY_WEBHOOK__RETRY_MAX_AGE="+c.age, "BOTHY_SYNC__EVENT_RETENTION="+c.retention)
		checkPairProblem(t, got, c.ok, "webhook.retry_max_age", "sync.event_retention", c.age, c.retention)
	}
}

// checkPairProblem checks the problems of a cross-check over two keys: none when ok, else a
// problem that names both keys and neither value.
func checkPairProblem(t *testing.T, got []config.Problem, ok bool, key1, key2, v1, v2 string) {
	t.Helper()
	if (len(got) == 0) != ok {
		t.Errorf("%s=%s %s=%s: problems %v, want ok %v", key1, v1, key2, v2, got, ok)
		return
	}
	if len(got) == 0 {
		return
	}
	msg := got[0].String()
	if !strings.Contains(msg, key1) || !strings.Contains(msg, key2) {
		t.Errorf("the problem does not name both keys: %q", msg)
	}
	for _, v := range []string{v1, v2} {
		if strings.Contains(msg, v) {
			t.Errorf("the problem names the value %s: %q", v, msg)
		}
	}
}

// TestConfig_JwksMaxAgeBelowRotationInterval: agent.jwks_max_age must be strictly below
// signing_key.invocation_rotation_interval and signing_key.access_token_rotation_interval
// (cross-check jwks_max_age_below_rotation, M1-32): a next key is published for agent.jwks_max_age
// before it signs, so a rotation interval not above it would rotate again before the last next
// key ever signed. The problem names the keys and no value.
func TestConfig_JwksMaxAgeBelowRotationInterval(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		maxAge, invocation, access string
		ok                         bool
		names                      string
	}{
		{"1h", "30d", "30d", true, ""},
		{"29d", "30d", "30d", true, ""},
		{"30d", "30d", "60d", false, "signing_key.invocation_rotation_interval"},
		{"31d", "60d", "30d", false, "signing_key.access_token_rotation_interval"},
		{"2h", "1h", "90m", false, "signing_key.invocation_rotation_interval"},
	} {
		got := load(t, "BOTHY_AGENT__JWKS_MAX_AGE="+c.maxAge, "BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL="+c.invocation,
			"BOTHY_SIGNING_KEY__ACCESS_TOKEN_ROTATION_INTERVAL="+c.access)
		if (len(got) == 0) != c.ok {
			t.Errorf("max age %s, intervals %s %s: problems %v, want ok %v", c.maxAge, c.invocation, c.access, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "agent.jwks_max_age") || !strings.Contains(msg, c.names) {
			t.Errorf("the problem does not name agent.jwks_max_age and %s: %q", c.names, msg)
		}
		for _, v := range []string{c.maxAge, c.invocation, c.access} {
			if strings.Contains(msg, " "+v) {
				t.Errorf("the problem names the value %s: %q", v, msg)
			}
		}
	}
}

// TestJobsSchemaIdentifier: jobs.schema must be an unquoted identifier other than public and
// without the prefix pg_ (cross-check jobs_schema); the problem names the key, never the value.
func TestJobsSchemaIdentifier(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"jobs", "_jobs", "jobs_it2", strings.Repeat("a", 63)} {
		if problems := load(t, "BOTHY_JOBS__SCHEMA="+ok); problems != nil {
			t.Errorf("jobs.schema %q: %v", ok, problems)
		}
	}
	for _, bad := range []string{"public", "information_schema", "pg_jobs", "Jobs", "2jobs", "jobs-x", `jobs"x`, "jobs.x", strings.Repeat("a", 64)} {
		problems := load(t, "BOTHY_JOBS__SCHEMA="+bad)
		if len(problems) != 1 || !strings.Contains(problems[0].Message, "jobs.schema") || strings.Contains(problems[0].Message, bad) {
			t.Errorf("jobs.schema %q: problems %v, want one naming the key and not the value", bad, problems)
		}
	}
}

// TestRateLimitConfig_InvalidValuesStopTheStart: a value below 1 never loads (exit code 2).
func TestRateLimitConfig_InvalidValuesStopTheStart(t *testing.T) {
	t.Parallel()
	for _, env := range []string{
		"BOTHY_RATELIMIT__RL__READ__PER_MINUTE=0", "BOTHY_RATELIMIT__RL__READ__BURST=0",
		"BOTHY_RATELIMIT__RL__HOST__BURST=-1", "BOTHY_RATELIMIT__MAX_TRACKED_KEYS=0",
		"BOTHY_RATELIMIT__SWEEP_INTERVAL=0s", "BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE=1000001",
	} {
		if problems := load(t, env); len(problems) == 0 {
			t.Errorf("%s loaded", env)
		}
	}
}

// TestConfigCheckDPoPRequiredFor: auth.dpop_required_for takes only agent, app and personal
// (cross-check dpop_required_for, M1-44); the problem names the key, never the value.
func TestConfigCheckDPoPRequiredFor(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"agent", "agent,app,personal", "", "personal"} {
		if problems := load(t, "BOTHY_AUTH__DPOP_REQUIRED_FOR="+ok); problems != nil {
			t.Errorf("auth.dpop_required_for %q: %v", ok, problems)
		}
	}
	for _, bad := range []string{"agents", "agent,people", "AGENT"} {
		got := load(t, "BOTHY_AUTH__DPOP_REQUIRED_FOR="+bad)
		if len(got) == 0 {
			t.Errorf("auth.dpop_required_for %q passed", bad)
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "auth.dpop_required_for") || strings.Contains(msg, bad) {
			t.Errorf("the problem must name the key and not the value %q: %q", bad, msg)
		}
	}
}

func TestClientVersionsAreSemVer(t *testing.T) {
	t.Parallel()
	for _, platform := range []string{"IOS", "ANDROID", "MACOS", "WINDOWS"} {
		for _, kind := range []string{"MIN_VERSION", "RECOMMENDED_VERSION"} {
			env := "BOTHY_CLIENT__" + kind + "__" + platform + "="
			if got := load(t, env+"1.2.3"); got != nil {
				t.Errorf("%s1.2.3: problems %v", env, got)
			}
			if got := load(t, env+"2.0.0-rc.1+build.5"); got != nil {
				t.Errorf("%s2.0.0-rc.1: problems %v", env, got)
			}
			for _, bad := range []string{"v1.2.3", "1.2", "latest", " 1.2.3"} {
				got := load(t, env+bad)
				if len(got) != 1 || !strings.Contains(got[0].Key+" "+got[0].Message, "client.") {
					t.Errorf("%s%q: problems %v, want one about the client versions", env, bad, got)
				}
				for _, p := range got {
					if strings.Contains(p.Message, bad) {
						t.Errorf("problem %q repeats the value", p.Message)
					}
				}
			}
		}
	}
}

func TestInstanceConfig_Defaults(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: append(testEnv(), "BOTHY_CLIENT__MIN_VERSION__IOS=1.0.0")})
	if err != nil {
		t.Fatal(err)
	}
	c, err := wire.InstanceConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if c.ProductNameMaxChars != 64 || c.MessageMaxContentBytes != 262144 || c.MessageMaxAttachments != 10 ||
		c.FileMaxUploadBytes != 104857600 || c.ThreadTitleMaxChars != 200 || c.VoiceMaxDuration.Minutes() != 15 {
		t.Errorf("config %+v", c)
	}
	if c.MinVersionIOS != "1.0.0" || c.MinVersionAndroid != "" || c.RecommendedVersionWindows != "" {
		t.Errorf("client versions %+v", c)
	}
	c.MinVersionIOS = ""
	if c != wire.DefaultInstanceConfig() {
		t.Errorf("DefaultInstanceConfig %+v, schema defaults %+v", wire.DefaultInstanceConfig(), c)
	}
}

// TestIDT24_Argon2BelowFloorStopsStart: each Argon2id key below its OWASP floor (19456 KiB, 2
// passes, parallelism 1; 08 section 2.5) stops the start and bothy config check with one problem
// that names the key; the floor values load (ID-T24, M1-39).
func TestIDT24_Argon2BelowFloorStopsStart(t *testing.T) {
	t.Parallel()
	for key, env := range map[string]string{
		"auth.argon2id_memory_kib":  "BOTHY_AUTH__ARGON2ID_MEMORY_KIB=19455",
		"auth.argon2id_iterations":  "BOTHY_AUTH__ARGON2ID_ITERATIONS=1",
		"auth.argon2id_parallelism": "BOTHY_AUTH__ARGON2ID_PARALLELISM=0",
	} {
		problems := load(t, env)
		if len(problems) != 1 || !strings.Contains(problems[0].Message+problems[0].Key, key) {
			t.Errorf("%s: problems %v, want one naming %s", env, problems, key)
		}
	}
	if problems := load(t, "BOTHY_AUTH__ARGON2ID_MEMORY_KIB=19456", "BOTHY_AUTH__ARGON2ID_ITERATIONS=2",
		"BOTHY_AUTH__ARGON2ID_PARALLELISM=1"); problems != nil {
		t.Fatalf("floor values: %v", problems)
	}
}

// TestSearchLanguagesCheck holds the check search_languages (D-M3-13-5): unique, unqualified
// lower-case identifiers, never simple. The problem names the key and the position, never the
// value.
func TestSearchLanguagesCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		value, secret string
		ok            bool
	}{
		{value: `["german","english"]`, ok: true},
		{value: `["english"]`, ok: true},
		{value: `["german","german"]`, secret: "german"},
		{value: `["German","english"]`, secret: "German"},
		{value: `["english","pt-br"]`, secret: "pt-br"},
		{value: `["9lang"]`, secret: "9lang"},
		{value: `["german","simple"]`},
		{value: `["` + strings.Repeat("x", 64) + `"]`, secret: strings.Repeat("x", 64)},
	} {
		got := load(t, "BOTHY_SEARCH__LANGUAGES="+c.value, `BOTHY_SEARCH__LOCALE_MAP={}`)
		if (len(got) == 0) != c.ok {
			t.Errorf("%s: problems %v, want ok %v", c.value, got, c.ok)
			continue
		}
		for _, p := range got {
			msg := p.String()
			if !strings.Contains(msg, "search_languages: search.languages: entry") {
				t.Errorf("%s: problem %q does not come from search_languages with a position", c.value, msg)
			}
			if c.secret != "" && strings.Contains(msg, c.secret) {
				t.Errorf("%s: problem %q names the value", c.value, msg)
			}
		}
	}
}

// TestSearchLocaleMapCheck holds the check search_locale_map (D-M3-13-5, 14 3.1): keys are first
// language subtags in lower-case ASCII, values valid names listed in search.languages or simple.
func TestSearchLocaleMapCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		value, secret string
		ok            bool
	}{
		{value: `{"de":"german","en":"english"}`, ok: true},
		{value: `{"de":"german","fr":"simple"}`, ok: true},
		{value: `{}`, ok: true},
		{value: `{"de":"german","fr":"french"}`, secret: "french"},
		{value: `{"de-AT":"german"}`, secret: "de-AT"},
		{value: `{"DE":"german"}`, secret: "DE"},
		{value: `{"d":"german"}`},
		{value: `{"de":"German"}`, secret: "German"},
		{value: `{"de":""}`},
	} {
		got := load(t, "BOTHY_SEARCH__LOCALE_MAP="+c.value)
		if (len(got) == 0) != c.ok {
			t.Errorf("%s: problems %v, want ok %v", c.value, got, c.ok)
			continue
		}
		for _, p := range got {
			msg := p.String()
			if !strings.Contains(msg, "search_locale_map: search.locale_map: ") || !strings.Contains(msg, "entry ") {
				t.Errorf("%s: problem %q does not come from search_locale_map with a position", c.value, msg)
			}
			if c.secret != "" && strings.Contains(msg, c.secret) {
				t.Errorf("%s: problem %q names the value", c.value, msg)
			}
		}
	}
	// A broken search.languages is reported once, by its own check.
	got := load(t, `BOTHY_SEARCH__LANGUAGES=["german","german"]`)
	if len(got) != 1 || !strings.Contains(got[0].String(), "search_languages:") {
		t.Errorf("problems %v, want one of search_languages", got)
	}
}

// TestConfig_FingerprintKeysChecked (M2-41, Design note 12): terminal.fingerprint_confirm_chars
// of 0 or above the length of the fingerprint and terminal.fingerprint_max_attempts of 0 are
// start errors that name the key.
func TestConfig_FingerprintKeysChecked(t *testing.T) {
	t.Parallel()
	if got := load(t, "BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS=52", "BOTHY_TERMINAL__FINGERPRINT_MAX_ATTEMPTS=1"); got != nil {
		t.Fatalf("the bounds are refused: %v", got)
	}
	for env, key := range map[string]string{
		"BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS=0":  "terminal.fingerprint_confirm_chars",
		"BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS=53": "terminal.fingerprint_confirm_chars",
		"BOTHY_TERMINAL__FINGERPRINT_MAX_ATTEMPTS=0":   "terminal.fingerprint_max_attempts",
	} {
		got := load(t, env)
		if len(got) == 0 || !strings.Contains(got[0].Key+" "+got[0].Message, key) {
			t.Errorf("%s: problems %v, want one naming %s", env, got, key)
		}
	}
}
