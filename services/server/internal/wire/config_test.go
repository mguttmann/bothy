// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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
		"terminal.step_up_max_age":              {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.quick_reattach_window":        {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "24-terminal.md"},
		"terminal.host_heartbeat_interval":      {Kind: config.KindDuration, Default: "20s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.host_heartbeat_miss_limit":    {Kind: config.KindInt, Default: "3", Min: "1", Chapter: "24-terminal.md"},
		"terminal.host_auth_timeout":            {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.host_duplicate_probe_timeout": {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "24-terminal.md"},
		"terminal.open_timeout":                 {Kind: config.KindDuration, Default: "20s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.relay_buffer_max_bytes":       {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "24-terminal.md"},
		"terminal.relay_stall_timeout":          {Kind: config.KindDuration, Default: "10s", Min: "1s", Chapter: "24-terminal.md"},
		"cluster.mode":                          {Kind: config.KindEnum, Default: "single", Enum: []string{"single", "multi"}, Chapter: "02-architecture.md"},
		"cluster.presence_flush_interval":       {Kind: config.KindDuration, Default: "2s", Min: "100ms", Chapter: "02-architecture.md"},
		"cluster.presence_ttl":                  {Kind: config.KindDuration, Default: "10s", Min: "1s", Chapter: "02-architecture.md"},
		"terminal.idle_timeout":                 {Kind: config.KindDuration, Default: "30m", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.detach_grace":                 {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.idle_grace":                   {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.max_session_duration":         {Kind: config.KindDuration, Default: "12h", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.max_sessions_per_person":      {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "24-terminal.md"},
		"terminal.max_sessions_per_host":        {Kind: config.KindInt, Default: "10", Min: "1", Chapter: "24-terminal.md"},
		"terminal.sweep_interval":               {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "24-terminal.md"},
		"audit.reason_max_chars":                {Kind: config.KindInt, Default: "500", Min: "1", Chapter: "19-security.md"},
		"host.name_max_chars":                   {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"host.os_user_max_chars":                {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-data-model.md"},
		"terminal.pairing_code_ttl":             {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.session_token_ttl":            {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "24-terminal.md"},
		"terminal.fingerprint_confirm_chars":    {Kind: config.KindInt, Default: "8", Min: "1", Chapter: "24-terminal.md"},
		"terminal.fingerprint_max_attempts":     {Kind: config.KindInt, Default: "3", Min: "1", Chapter: "24-terminal.md"},
		"terminal.report_label_max_chars":       {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "24-terminal.md"},
		"terminal.report_max_entries":           {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "24-terminal.md"},
		"server.role":                           {Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"}, Chapter: "16-operations.md"},
		"server.listen_addr":                    {Kind: config.KindListenAddr, Default: "127.0.0.1:8080", Chapter: "16-operations.md"},
		"server.internal_listen_addr":           {Kind: config.KindListenAddr, Default: "127.0.0.1:8081", Chapter: "16-operations.md"},
		"server.maintenance_mode":               {Kind: config.KindEnum, Default: "off", Enum: []string{"off", "read_only"}, Chapter: "16-operations.md"},
		"server.maintenance_retry_after":        {Kind: config.KindDuration, Default: "1m", Min: "1s", Max: "1h", Chapter: "16-operations.md"},
		"server.tls_cert_ref":                   {Kind: config.KindSecretRef, Default: "", Chapter: "16-operations.md"},
		"server.tls_key_ref":                    {Kind: config.KindSecretRef, Default: "", Chapter: "16-operations.md"},
		"http.hsts_max_age":                     {Kind: config.KindDuration, Default: "31536000s", Min: "0s", Chapter: "19-security.md"},
		"http.max_json_body_bytes":              {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "02-architecture.md"},
		"http.read_header_timeout":              {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.idle_timeout":                     {Kind: config.KindDuration, Default: "120s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.max_header_bytes":                 {Kind: config.KindInt, Default: "65536", Min: "4096", Max: "16777216", Chapter: "02-architecture.md"},
		"http.max_json_depth":                   {Kind: config.KindInt, Default: "32", Min: "1", Max: "1000", Chapter: "02-architecture.md"},
		"http.request_timeout":                  {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.transfer_timeout":                 {Kind: config.KindDuration, Default: "10m", Min: "1ms", Chapter: "02-architecture.md"},
		"http.unavailable_retry_after":          {Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},
		"file.max_upload_bytes":                 {Kind: config.KindInt, Default: "104857600", Min: "1", Chapter: "03-data-model.md"},
		"file.storage.driver":                   {Kind: config.KindEnum, Default: "fs", Enum: []string{"fs", "s3"}, Chapter: "12-files.md"},
		"file.fs.root":                          {Kind: config.KindString, Default: "/var/lib/bothy/files", Chapter: "12-files.md"},
		"file.s3.endpoint":                      {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.region":                        {Kind: config.KindString, Default: "us-east-1", Chapter: "12-files.md"},
		"file.s3.bucket":                        {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.prefix":                        {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.use_path_style":                {Kind: config.KindBool, Default: "true", Chapter: "12-files.md"},
		"file.s3.checksum_mode":                 {Kind: config.KindEnum, Default: "when_required", Enum: []string{"when_required", "when_supported"}, Chapter: "12-files.md"},
		"file.s3.access_key_ref":                {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_access_key", Chapter: "12-files.md"},
		"file.s3.secret_key_ref":                {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_secret_key", Chapter: "12-files.md"},
		"file.fs.min_free_bytes":                {Kind: config.KindInt, Default: "2147483648", Min: "0", Chapter: "12-files.md"},
		"file.upload_part_bytes":                {Kind: config.KindInt, Default: "8388608", Min: "1", Chapter: "12-files.md"},
		"file.upload_session_ttl":               {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "12-files.md"},
		"file.unattached_ttl":                   {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "12-files.md"},
		"file.failed_row_ttl":                   {Kind: config.KindDuration, Default: "7d", Min: "1s", Chapter: "12-files.md"},
		"file.gc_interval":                      {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "12-files.md"},
		"file.gc_batch_size":                    {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "12-files.md"},
		"privacy.object_delete_delay":           {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "20-privacy.md"},
		"file.upload_max_open_sessions":         {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "12-files.md"},
		"file.quota_bytes_per_principal":        {Kind: config.KindInt, Default: "0", Min: "0", Chapter: "12-files.md"},
		"file.name_max_bytes":                   {Kind: config.KindInt, Default: "255", Min: "4", Chapter: "12-files.md"},
		"file.denied_mime_types":                {Kind: config.KindStringList, Default: "", Chapter: "12-files.md"},
		"file.s3.presigned_upload":              {Kind: config.KindBool, Default: "false", Chapter: "12-files.md"},
		"file.scan.mode":                        {Kind: config.KindEnum, Default: "required", Enum: []string{"required", "off"}, Chapter: "12-files.md"},
		"file.scan.clamd_address":               {Kind: config.KindString, Default: "unix:///run/clamav/clamd.sock", Chapter: "12-files.md"},
		"file.scan.chunk_bytes":                 {Kind: config.KindInt, Default: "65536", Min: "1", Max: "2147483647", Chapter: "12-files.md"},
		"file.scan.timeout":                     {Kind: config.KindDuration, Default: "120s", Min: "1s", Chapter: "12-files.md"},
		"file.invocation_hold_max":              {Kind: config.KindDuration, Default: "120s", Min: "1s", Chapter: "12-files.md"},
		"file.scan.max_concurrent":              {Kind: config.KindInt, Default: "4", Min: "1", Chapter: "12-files.md"},
		"file.scan.retry_backoff_cap":           {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "12-files.md"},
		"file.scan.health_interval":             {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "12-files.md"},
		"file.scan.clamd_stream_max_bytes":      {Kind: config.KindInt, Default: "104857600", Min: "1", Chapter: "12-files.md"},
		"file.sniff_bytes":                      {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "12-files.md"},
		"file.ticket_ttl":                       {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "12-files.md"},
		"file.ticket_key_ref":                   {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/file_ticket_key", Chapter: "12-files.md"},
		"file.not_ready_retry_after":            {Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h", Chapter: "12-files.md"},
		"client.cache_max_age":                  {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "04-sync-protocol.md"},
		"file.image.max_pixels":                 {Kind: config.KindInt, Default: "50000000", Min: "1", Chapter: "12-files.md"},
		"file.image.jpeg_quality":               {Kind: config.KindInt, Default: "90", Min: "1", Max: "100", Chapter: "12-files.md"},
		"file.image.processing_timeout":         {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "12-files.md"},
		"file.image.max_concurrent":             {Kind: config.KindInt, Default: "2", Min: "1", Chapter: "12-files.md"},
		"file.thumbnail.small_px":               {Kind: config.KindInt, Default: "320", Min: "1", Chapter: "12-files.md"},
		"file.thumbnail.medium_px":              {Kind: config.KindInt, Default: "1024", Min: "1", Chapter: "12-files.md"},
		"file.thumbnail.large_px":               {Kind: config.KindInt, Default: "2048", Min: "1", Chapter: "12-files.md"},
		"file.thumbnail.jpeg_quality":           {Kind: config.KindInt, Default: "82", Min: "1", Max: "100", Chapter: "12-files.md"},
		"api.cors_allowed_origins":              {Kind: config.KindStringList, Default: "", Chapter: "06-api.md"},
		"server.public_url":                     {Kind: config.KindURL, Required: true, Chapter: "16-operations.md"},
		"server.trusted_proxies":                {Kind: config.KindCIDRList, Default: "127.0.0.1/32, ::1/128", Chapter: "16-operations.md"},
		"shutdown.drain_delay":                  {Kind: config.KindDuration, Default: "5s", Min: "0s", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.http_grace":                   {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.telemetry_flush_timeout":      {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.total_timeout":                {Kind: config.KindDuration, Default: "60s", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.jobs_soft_timeout":            {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"jobs.schema":                           {Kind: config.KindString, Default: "jobs", Chapter: "02-architecture.md"},
		"jobs.queue.default.max_workers":        {Kind: config.KindInt, Default: "8", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.files.max_workers":          {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.delivery.max_workers":       {Kind: config.KindInt, Default: "16", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.maintenance.max_workers":    {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.privacy.max_workers":        {Kind: config.KindInt, Default: "1", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"message.max_content_bytes":             {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "03-data-model.md"},
		"principal.handle_max_chars":            {Kind: config.KindInt, Default: "32", Min: "1", Chapter: "03-data-model.md"},
		"principal.display_name_max_chars":      {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-data-model.md"},
		"auth.dpop_nonce_ttl":                   {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "08-identity.md"},
		"auth.session_max_age":                  {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "03-data-model.md"},
		"auth.refresh_token_ttl":                {Kind: config.KindDuration, Default: "14d", Min: "1m", Chapter: "03-data-model.md"},
		"auth.console_session_max_age":          {Kind: config.KindDuration, Default: "12h", Min: "1m", Chapter: "08-identity.md"},
		"auth.console_session_idle_timeout":     {Kind: config.KindDuration, Default: "30m", Min: "1m", Chapter: "08-identity.md"},
		"auth.refresh_reuse_grace":              {Kind: config.KindDuration, Default: "30s", Min: "0s", Chapter: "03-data-model.md"},
		"identity.last_active_granularity":      {Kind: config.KindDuration, Default: "5m", Min: "0s", Chapter: "03-data-model.md"},
		"auth.step_up_challenge_ttl":            {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "08-identity.md"},
		"auth.step_up_fail_limit":               {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "08-identity.md"},
		"auth.login_fail_window":                {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "08-identity.md"},
		"auth.cleanup_interval":                 {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "08-identity.md"},
		"auth.device_pending_ttl":               {Kind: config.KindDuration, Default: "24h", Min: "1m", Chapter: "08-identity.md"},
		"auth.device_verification_code_digits":  {Kind: config.KindInt, Default: "6", Min: "6", Max: "12", Chapter: "08-identity.md"},
		"auth.pairing_code_ttl":                 {Kind: config.KindDuration, Default: "5m", Min: "30s", Chapter: "08-identity.md"},
		"auth.pairing_code_chars":               {Kind: config.KindInt, Default: "8", Min: "8", Max: "16", Chapter: "08-identity.md"},
		"auth.enrollment_code_ttl":              {Kind: config.KindDuration, Default: "72h", Min: "1m", Chapter: "08-identity.md"},
		"auth.enrollment_code_max_attempts":     {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "08-identity.md"},
		"auth.dpop_proof_max_age":               {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "08-identity.md"},
		"auth.clock_leeway":                     {Kind: config.KindDuration, Default: "30s", Min: "0s", Chapter: "08-identity.md"},
		"auth.token_pepper_ref":                 {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper", Chapter: "08-identity.md"},
		"auth.dpop_required_for":                {Kind: config.KindStringList, Default: `["agent"]`, Chapter: "08-identity.md"},
		"auth.api_token_max_ttl":                {Kind: config.KindDuration, Default: "365d", Min: "1s", Chapter: "08-identity.md"},
		"auth.personal_token_max_ttl":           {Kind: config.KindDuration, Default: "90d", Min: "1s", Chapter: "08-identity.md"},
		"api.page_size_default":                 {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "06-api.md"},
		"api.page_size_max":                     {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "06-api.md"},
		"authz.snapshot_cache_max_entries":      {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "07-permissions.md"},
		"authz.expiry_sweep_interval":           {Kind: config.KindDuration, Default: "1m", Min: "1s", Chapter: "07-permissions.md"},
		"authz.expiry_reminder_lead":            {Kind: config.KindDuration, Default: "48h", Min: "1s", Chapter: "07-permissions.md"},
		"authz.expiry_sweep_batch_max":          {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "07-permissions.md"},
		"authz.view_as_max_duration":            {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "07-permissions.md"},
		"auth.step_up_max_age":                  {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "03-data-model.md"},
		"group.name_max_chars":                  {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"group.description_max_chars":           {Kind: config.KindInt, Default: "500", Min: "1", Chapter: "03-data-model.md"},
		"space.name_max_chars":                  {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"channel.name_max_chars":                {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"channel.topic_max_chars":               {Kind: config.KindInt, Default: "1024", Min: "1", Chapter: "03-data-model.md"},
		"forum_tag.name_max_chars":              {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-data-model.md"},
		"thread.max_tags":                       {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "03-data-model.md"},
		"sidebar.order_max_items":               {Kind: config.KindInt, Default: "1000", Min: "1", Max: "1000", Chapter: "03-data-model.md"},
		"chat.delete_undo_window":               {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "03-data-model.md"},
		"chat.delete_purge_interval":            {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "03-data-model.md"},
		"thread.placeholder_titles": {Kind: config.KindStringList, Default: `["Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"]`,
			Chapter: "09-agents.md"},
		"channel.history_backfill_limit_default": {Kind: config.KindInt, Default: "20", Min: "0", Chapter: "03-data-model.md"},
		"channel.history_backfill_limit_max":     {Kind: config.KindInt, Default: "200", Min: "0", Chapter: "03-data-model.md"},
		"channel.prompt_max_bytes":               {Kind: config.KindInt, Default: "32768", Min: "1", Chapter: "03-data-model.md"},
		"project.binding_text_max_chars":         {Kind: config.KindInt, Default: "512", Min: "1", Max: "600", Chapter: "09-agents.md"},
		"egress.allowed_hosts":                   {Kind: config.KindStringList, Default: "", Chapter: "19-security.md"},
		"egress.timeout":                         {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "19-security.md"},
		"egress.max_response_bytes":              {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "19-security.md"},
		"secrets.dir":                            {Kind: config.KindString, Default: "/run/bothy/secrets", Chapter: "16-operations.md"},
		"secrets.cache_ttl":                      {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "16-operations.md"},
		"secrets.signing_key_dir":                {Kind: config.KindString, Default: "/var/lib/bothy/signing-keys", Chapter: "16-operations.md"},
		"console.assets_dir":                     {Kind: config.KindString, Default: "", Chapter: "16-operations.md"},
		"console.csp_report_max_records":         {Kind: config.KindInt, Default: "20", Min: "1", Max: "1000", Chapter: "16-operations.md"},
		"console.grafana_embed_url":              {Kind: config.KindString, Default: "", Chapter: "25-visualization.md"},

		"signing_key.cache_ttl":                      {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "19-security.md"},
		"signing_key.accept_lost_keys":               {Kind: config.KindBool, Default: "false", Chapter: "19-security.md"},
		"signing_key.rotation_check_interval":        {Kind: config.KindDuration, Default: "1m", Min: "1s", Chapter: "19-security.md"},
		"signing_key.access_token_rotation_interval": {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "19-security.md"},
		"signing_key.invocation_rotation_interval":   {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "09-agents.md"},
		"agent.jwks_max_age":                         {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "09-agents.md"},
		"agent.invocation_token_ttl":                 {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "04-sync-protocol.md"},
		"auth.access_token_ttl":                      {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "08-identity.md"},
		"webhook.delivery_timeout":                   {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "11-extensions.md"},

		"database.host":                         {Kind: config.KindString, Default: "localhost", Chapter: "16-operations.md"},
		"database.port":                         {Kind: config.KindInt, Default: "5432", Min: "1", Max: "65535", Chapter: "16-operations.md"},
		"database.name":                         {Kind: config.KindString, Default: "bothy", Chapter: "16-operations.md"},
		"database.user":                         {Kind: config.KindString, Default: "bothy_app", Chapter: "16-operations.md"},
		"database.password_ref":                 {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-operations.md"},
		"database.sslmode":                      {Kind: config.KindEnum, Default: "verify-full", Enum: []string{"disable", "verify-full"}, Chapter: "16-operations.md"},
		"database.sslrootcert":                  {Kind: config.KindString, Default: "", Chapter: "16-operations.md"},
		"database.max_conns":                    {Kind: config.KindInt, Default: "20", Min: "1", Max: "2147483647", Chapter: "16-operations.md"},
		"database.restore_guard":                {Kind: config.KindEnum, Default: "on", Enum: []string{"on", "off"}, Chapter: "16-operations.md"},
		"database.acquire_timeout":              {Kind: config.KindDuration, Default: "2s", Min: "1ms", Chapter: "02-architecture.md"},
		"database.statement_timeout":            {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.lock_timeout":                 {Kind: config.KindDuration, Default: "2s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.idle_in_tx_timeout":           {Kind: config.KindDuration, Default: "10s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.tx_retry_max":                 {Kind: config.KindInt, Default: "2", Min: "0", Chapter: "02-architecture.md"},
		"database.listen_reconnect_backoff_cap": {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "02-architecture.md"},
		"startup.dependency_wait":               {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "02-architecture.md"},
		"migrate.lock_timeout":                  {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "16-operations.md"},
		"migrate.bootstrap_user":                {Kind: config.KindString, Default: "postgres", Chapter: "16-operations.md"},
		"migrate.bootstrap_password_ref":        {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_superuser_password", Chapter: "16-operations.md"},
		"migrate.owner_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_owner_password", Chapter: "16-operations.md"},
		"migrate.app_password_ref":              {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-operations.md"},
		"migrate.maint_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_maint_password", Chapter: "16-operations.md"},
		"migrate.monitor_password_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_monitor_password", Chapter: "16-operations.md"},
		"migrate.backup_password_ref":           {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_backup_password", Chapter: "16-operations.md"},
		"migrate.require_recent_backup":         {Kind: config.KindBool, Default: "true", Chapter: "16-operations.md"},
		"migrate.recent_backup_max_age":         {Kind: config.KindDuration, Default: "24h", Min: "1m", Chapter: "16-operations.md"},
		"backup.pg.full_schedule":               {Kind: config.KindString, Default: "0 2 * * 0", Chapter: "16-operations.md"},
		"backup.pg.diff_schedule":               {Kind: config.KindString, Default: "0 2 * * 1-6", Chapter: "16-operations.md"},
		"backup.pg.retention_full":              {Kind: config.KindInt, Default: "4", Min: "1", Chapter: "16-operations.md"},
		"backup.pg.local_retention_full":        {Kind: config.KindInt, Default: "1", Min: "1", Chapter: "16-operations.md"},
		"backup.logical.schedule":               {Kind: config.KindString, Default: "0 3 * * *", Chapter: "16-operations.md"},
		"backup.logical.retention":              {Kind: config.KindDuration, Default: "30d", Min: "1d", Chapter: "16-operations.md"},
		"backup.objects.schedule":               {Kind: config.KindString, Default: "30 3 * * *", Chapter: "16-operations.md"},
		"backup.objects.retention":              {Kind: config.KindDuration, Default: "90d", Min: "1d", Chapter: "16-operations.md"},
		"backup.offsite.target":                 {Kind: config.KindString, Default: "", Chapter: "16-operations.md"},
		"backup.offsite.region":                 {Kind: config.KindString, Default: "us-east-1", Chapter: "16-operations.md"},
		"backup.offsite.use_path_style":         {Kind: config.KindBool, Default: "true", Chapter: "16-operations.md"},
		"backup.restore_test.schedule":          {Kind: config.KindString, Default: "0 5 * * 6", Chapter: "16-operations.md"},
		"backup.restore_test.sample_files":      {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "16-operations.md"},
		"backup.restore_test.sample_threads":    {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "16-operations.md"},
		"dr.rto_target":                         {Kind: config.KindDuration, Default: "4h", Min: "1s", Chapter: "16-operations.md"},

		"instance.environment":                 {Kind: config.KindEnum, Default: "prod", Enum: []string{"dev", "staging", "prod"}, Chapter: "16-operations.md"},
		"telemetry.enabled":                    {Kind: config.KindBool, Default: "true", Chapter: "17-observability.md"},
		"telemetry.traces.sampler_ratio":       {Kind: config.KindFloat, Default: "1.0", Min: "0", Max: "1", Chapter: "17-observability.md"},
		"telemetry.traces.trust_remote_parent": {Kind: config.KindBool, Default: "false", Chapter: "17-observability.md"},
		"telemetry.logs.exporter":              {Kind: config.KindEnum, Default: "both", Enum: []string{"otlp", "stdout", "both"}, Chapter: "17-observability.md"},
		"telemetry.logs.level":                 {Kind: config.KindEnum, Default: "info", Enum: []string{"debug", "info", "warn", "error"}, Chapter: "17-observability.md"},
		"telemetry.pseudonym_key_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/telemetry_pseudonym_key", Chapter: "17-observability.md"},
		"telemetry.dashboard_url":              {Kind: config.KindString, Default: "", Chapter: "17-observability.md"},
		"privacy.telemetry_pseudonym_rotation": {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-privacy.md"},

		"audit.deny_aggregate_window":        {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "05-actions.md"},
		"audit.deny_aggregate_max_keys":      {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "05-actions.md"},
		"audit.checkpoint_interval":          {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "19-security.md"},
		"audit.checkpoint_sink":              {Kind: config.KindString, Default: "", Chapter: "19-security.md"},
		"audit.checkpoint_s3_endpoint":       {Kind: config.KindString, Default: "", Chapter: "19-security.md"},
		"audit.checkpoint_s3_region":         {Kind: config.KindString, Default: "us-east-1", Chapter: "19-security.md"},
		"audit.checkpoint_s3_use_path_style": {Kind: config.KindBool, Default: "true", Chapter: "19-security.md"},
		"audit.checkpoint_s3_access_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_checkpoint_s3_access_key", Chapter: "19-security.md"},
		"audit.checkpoint_s3_secret_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_checkpoint_s3_secret_key", Chapter: "19-security.md"},
		"retention.audit":                    {Kind: config.KindDuration, Default: "365d", Min: "1d", Chapter: "20-privacy.md"},
		"retention.content":                  {Kind: config.KindString, Default: "none", Chapter: "20-privacy.md"},
		"retention.content_none_reason":      {Kind: config.KindString, Default: "", Chapter: "20-privacy.md"},
		"retention.security_after_end":       {Kind: config.KindDuration, Default: "90d", Min: "1d", Chapter: "20-privacy.md"},
		"retention.cost":                     {Kind: config.KindDuration, Default: "395d", Min: "1d", Chapter: "20-privacy.md"},
		"retention.webhook_deliveries":       {Kind: config.KindDuration, Default: "30d", Min: "1d", Chapter: "20-privacy.md"},
		"retention.job_interval":             {Kind: config.KindDuration, Default: "1h", Min: "1m", Chapter: "20-privacy.md"},
		"retention.batch_size":               {Kind: config.KindInt, Default: "500", Min: "1", Max: "10000", Chapter: "20-privacy.md"},
		"privacy.audit_ip_key_period":        {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-privacy.md"},
		"privacy.audit_ip_key_retention":     {Kind: config.KindDuration, Default: "90d", Min: "1s", Chapter: "20-privacy.md"},
		"privacy.audit_ip_kek_ref":           {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_ip_kek", Chapter: "20-privacy.md"},
		"ratelimit.max_tracked_keys":         {Kind: config.KindInt, Default: "100000", Min: "1", Max: "100000000", Chapter: "02-architecture.md"},
		"ratelimit.sweep_interval":           {Kind: config.KindDuration, Default: "1m", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},

		"sync.event_log_partition_size":       {Kind: config.KindInt, Default: "1000000", Min: "1", Max: "1000000000000", Chapter: "03-data-model.md"},
		"sync.event_retention":                {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "04-sync-protocol.md"},
		"sync.event_payload_max_bytes":        {Kind: config.KindInt, Default: "65536", Min: "3", Max: "1048576", Chapter: "04-sync-protocol.md"},
		"sync.log_poll_interval":              {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "02-architecture.md"},
		"sync.tail_batch_size":                {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architecture.md"},
		"sync.partition_maintenance_interval": {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "02-architecture.md"},
		"sync.idempotency_key_ttl":            {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "04-sync-protocol.md"},
		"sync.idempotency_cleanup_interval":   {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "02-architecture.md"},
		"sync.idempotency_cleanup_batch_size": {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architecture.md"},
		"sync.event_log_partitions_ahead":     {Kind: config.KindInt, Default: "3", Min: "1", Max: "1000", Chapter: "02-architecture.md"},
		"sync.catchup_page_size":              {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "04-sync-protocol.md"},
		"sync.catchup_max_events":             {Kind: config.KindInt, Default: "50000", Min: "1", Max: "1000000000000", Chapter: "04-sync-protocol.md"},
		"sync.cursor_advance_every":           {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "04-sync-protocol.md"},
		"sync.snapshot_thread_window":         {Kind: config.KindDuration, Default: "7d", Min: "1s", Chapter: "04-sync-protocol.md"},
		"sync.snapshot_threads_per_channel":   {Kind: config.KindInt, Default: "20", Min: "1", Max: "1000", Chapter: "04-sync-protocol.md"},

		"consumer.batch_size":          {Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: "02-architecture.md"},
		"consumer.poll_interval":       {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "02-architecture.md"},
		"consumer.pending_alert_after": {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "02-architecture.md"},

		"branding.product_name_max_chars": {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-data-model.md"},
		"message.max_attachments":         {Kind: config.KindInt, Default: "10", Min: "0", Chapter: "03-data-model.md"},
		// M1-56: the mention limit of sending.
		"message.max_mentions": {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-data-model.md"},
		// M1-57: the content limit of the history lists.
		"message.list_content_max_bytes":    {Kind: config.KindInt, Default: "16384", Min: "1", Chapter: "06-api.md"},
		"thread.title_max_chars":            {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "03-data-model.md"},
		"template.name_max_chars":           {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"template.max_per_scope":            {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "03-data-model.md"},
		"reaction.emoji_max_bytes":          {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-data-model.md"},
		"reaction.max_distinct_per_message": {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "03-data-model.md"},
		"pin.max_per_thread":                {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "03-data-model.md"},
		"voice.max_duration":                {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "12-files.md"},
		// M3-09: the transcript of voice messages (TestVoiceConfigKeys checks the same keys).
		"voice.transcription_mode":           {Kind: config.KindEnum, Default: "agent", Enum: []string{"off", "agent", "service"}, Chapter: "12-files.md"},
		"voice.transcription_url":            {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"voice.transcription_api_key_ref":    {Kind: config.KindSecretRef, Default: "", Chapter: "12-files.md"},
		"voice.transcription_timeout":        {Kind: config.KindDuration, Default: "120s", Min: "1s", Chapter: "12-files.md"},
		"voice.transcript_hold_max":          {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "12-files.md"},
		"voice.transcription_model":          {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"voice.transcription_max_attempts":   {Kind: config.KindInt, Default: "5", Min: "1", Chapter: "12-files.md"},
		"auth.local_password_enabled":        {Kind: config.KindBool, Default: "true", Chapter: "08-identity.md"},
		"auth.login_flow_ttl":                {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "08-identity.md"},
		"auth.webauthn_rp_id":                {Kind: config.KindString, Default: "", Chapter: "08-identity.md"},
		"auth.webauthn_origins":              {Kind: config.KindStringList, Default: "", Chapter: "08-identity.md"},
		"auth.server_code_ttl":               {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "08-identity.md"},
		"auth.oidc_jwks_cache_ttl":           {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "08-identity.md"},
		"auth.oidc_client_assertion_ttl":     {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "08-identity.md"},
		"auth.device_inactive_revoke_after":  {Kind: config.KindDuration, Default: "180d", Min: "1h", Chapter: "08-identity.md"},
		"auth.trusted_header_bind_requests":  {Kind: config.KindBool, Default: "true", Chapter: "08-identity.md"},
		"auth.idp_recheck_interval":          {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "08-identity.md"},
		"auth.group_claim_max_age":           {Kind: config.KindDuration, Default: "30d", Min: "1m", Chapter: "08-identity.md"},
		"auth.argon2id_memory_kib":           {Kind: config.KindInt, Default: "65536", Min: "19456", Max: "4194304", Chapter: "08-identity.md"},
		"auth.argon2id_iterations":           {Kind: config.KindInt, Default: "3", Min: "2", Max: "1000", Chapter: "08-identity.md"},
		"auth.argon2id_parallelism":          {Kind: config.KindInt, Default: "1", Min: "1", Max: "255", Chapter: "08-identity.md"},
		"auth.password_min_length":           {Kind: config.KindInt, Default: "12", Min: "1", Max: "1024", Chapter: "08-identity.md"},
		"auth.login_fail_limit":              {Kind: config.KindInt, Default: "10", Min: "1", Chapter: "08-identity.md"},
		"auth.login_lock_duration":           {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "08-identity.md"},
		"auth.totp_key_ref":                  {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/totp_key", Chapter: "08-identity.md"},
		"client.app_lock_after":              {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "08-identity.md"},
		"security.app_lock_required":         {Kind: config.KindBool, Default: "false", Chapter: "08-identity.md"},
		"client.min_version.ios":             {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.android":         {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.macos":           {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.min_version.windows":         {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.ios":     {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.android": {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.macos":   {Kind: config.KindString, Default: "", Chapter: "18-release.md"},
		"client.recommended_version.windows": {Kind: config.KindString, Default: "", Chapter: "18-release.md"},

		// M2-24
		"app.manifest_max_bytes": {Kind: config.KindInt, Default: "65536", Min: "1", Chapter: "11-extensions.md"},
		"webhook.allowed_hosts":  {Kind: config.KindStringList, Default: "", Chapter: "03-data-model.md"},
		// M2-25
		"app.interaction_timeout":       {Kind: config.KindDuration, Default: "3s", Min: "1ms", Chapter: "11-extensions.md"},
		"command.max_per_principal":     {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "09-agents.md"},
		"command.description_max_chars": {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "09-agents.md"},
		// M2-27
		"webhook_in.max_body_bytes":  {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "11-extensions.md"},
		"webhook_in.formats_enabled": {Kind: config.KindStringList, Default: `["native", "alertmanager"]`, Chapter: "11-extensions.md"},
		// M2-16
		"command.autocomplete_max_choices": {Kind: config.KindInt, Default: "25", Min: "1", Max: "25", Chapter: "09-agents.md"},
		// M2-17
		"command.autocomplete_timeout": {Kind: config.KindDuration, Default: "2500ms", Min: "1ms", Chapter: "09-agents.md"},

		// M3-13
		"search.languages":  {Kind: config.KindStringList, Default: `["german","english"]`, Chapter: "14-search.md"},
		"search.locale_map": {Kind: config.KindStringMap, Default: `{"de":"german","en":"english"}`, Chapter: "14-search.md"},
		// The query side of the search (M3-14, brief design note 12).
		"search.query_max_chars":          {Kind: config.KindInt, Default: "256", Min: "1", Chapter: "14-search.md"},
		"search.max_candidates":           {Kind: config.KindInt, Default: "2000", Min: "1", Max: "1048575", Chapter: "14-search.md"},
		"search.rank_normalization":       {Kind: config.KindInt, Default: "1", Min: "0", Max: "63", Chapter: "14-search.md"},
		"search.recency_scale":            {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "14-search.md"},
		"search.page_size_default":        {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "14-search.md"},
		"search.page_size_max":            {Kind: config.KindInt, Default: "50", Min: "1", Max: "1048575", Chapter: "14-search.md"},
		"search.headline_input_max_chars": {Kind: config.KindInt, Default: "20000", Min: "1", Max: "1048575", Chapter: "14-search.md"},
		"search.headline_max_fragments":   {Kind: config.KindInt, Default: "2", Min: "0", Max: "2147483647", Chapter: "14-search.md"},
		"search.headline_max_words":       {Kind: config.KindInt, Default: "30", Min: "1", Max: "2147483647", Chapter: "14-search.md"},
		"search.headline_min_words":       {Kind: config.KindInt, Default: "10", Min: "1", Max: "2147483647", Chapter: "14-search.md"},
		"search.prefix_min_chars":         {Kind: config.KindInt, Default: "2", Min: "1", Max: "2147483647", Chapter: "14-search.md"},
		"search.statement_timeout":        {Kind: config.KindDuration, Default: "3s", Min: "1ms", Max: "24d", Chapter: "14-search.md"},
		// The text extraction of files (M3-15, brief design note 12).
		"search.extract.max_input_bytes":        {Kind: config.KindInt, Default: "52428800", Min: "1", Chapter: "14-search.md"},
		"search.extract.max_text_bytes":         {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "14-search.md"},
		"search.extract.max_uncompressed_bytes": {Kind: config.KindInt, Default: "209715200", Min: "1", Chapter: "14-search.md"},
		"search.extract.max_archive_entries":    {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "14-search.md"},
		"search.extract.max_xml_depth":          {Kind: config.KindInt, Default: "256", Min: "1", Chapter: "14-search.md"},
		"search.extract.timeout":                {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "14-search.md"},
		"search.extract.refresh_interval":       {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "14-search.md"},
		"search.extract.refresh_batch_size":     {Kind: config.KindInt, Default: "500", Min: "1", Chapter: "14-search.md"},

		// M2-26
		"webhook.max_consecutive_failures":     {Kind: config.KindInt, Default: "20", Min: "1", Chapter: "03-data-model.md"},
		"webhook.retry_backoff_base":           {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "03-data-model.md"},
		"webhook.retry_backoff_cap":            {Kind: config.KindDuration, Default: "1h", Min: "1ms", Chapter: "03-data-model.md"},
		"webhook.max_pending_per_subscription": {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "11-extensions.md"},
		"webhook.retry_max_age":                {Kind: config.KindDuration, Default: "72h", Min: "1ms", Chapter: "11-extensions.md"},
		"webhook.secret_rotation_overlap":      {Kind: config.KindDuration, Default: "24h", Min: "1ms", Chapter: "11-extensions.md"},
		"webhook.hmac_master_key_ref": {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/webhook_hmac_master_key",
			Chapter: "11-extensions.md"},
		// M3-18
		"notify.user_default_level": {Kind: config.KindEnum, Default: "mentions", Enum: []string{"all", "mentions", "mute"}, Chapter: "13-push-notifications.md"},
		// M3-19
		"notify.coalesce_window":                         {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "13-push-notifications.md"},
		"notify.mobile_delay_when_desktop_active":        {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "13-push-notifications.md"},
		"notify.default_admin_enabled":                   {Kind: config.KindBool, Default: "true", Chapter: "13-push-notifications.md"},
		"notify.default_input_needed_breaks_quiet_hours": {Kind: config.KindBool, Default: "false", Chapter: "13-push-notifications.md"},
		"notify.badge_sync_min_interval":                 {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "13-push-notifications.md"},
		"notify.default_preview_mode":                    {Kind: config.KindEnum, Default: "full", Enum: []string{"full", "names", "none"}, Chapter: "13-push-notifications.md"},
		"notify.preview_max_chars":                       {Kind: config.KindInt, Default: "240", Min: "1", Max: "1000", Chapter: "13-push-notifications.md"},
		"push.registration_field_max_bytes":              {Kind: config.KindInt, Default: "1024", Min: "64", Max: "2048", Chapter: "13-push-notifications.md"},
		"push.max_age":                                   {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "13-push-notifications.md"},
		"push.retry_base":                                {Kind: config.KindDuration, Default: "2s", Min: "1ms", Chapter: "13-push-notifications.md"},
		"push.retry_cap":                                 {Kind: config.KindDuration, Default: "5m", Min: "1ms", Chapter: "13-push-notifications.md"},
		"push.max_concurrent_requests":                   {Kind: config.KindInt, Default: "32", Min: "1", Max: "100000", Chapter: "13-push-notifications.md"},
		"push.max_consecutive_failures":                  {Kind: config.KindInt, Default: "5", Min: "1", Max: "100000", Chapter: "03-data-model.md"},
		"push.ttl":                                       {Kind: config.KindDuration, Default: "24h", Min: "1s", Max: "672h", Chapter: "13-push-notifications.md"},
		"push.apns.team_id":                              {Kind: config.KindString, Default: "", Chapter: "13-push-notifications.md"},
		"push.apns.key_id":                               {Kind: config.KindString, Default: "", Chapter: "13-push-notifications.md"},
		"push.apns.private_key_ref":                      {Kind: config.KindSecretRef, Default: "", Chapter: "13-push-notifications.md"},
		"push.apns.topics":                               {Kind: config.KindStringList, Default: "", Chapter: "13-push-notifications.md"},
		"push.apns.jwt_refresh_interval":                 {Kind: config.KindDuration, Default: "40m", Min: "20m", Max: "60m", Chapter: "13-push-notifications.md"},
		"push.apns.retry_5xx_delay":                      {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: "13-push-notifications.md"},
		"push.apns.time_sensitive_enabled":               {Kind: config.KindBool, Default: "false", Chapter: "13-push-notifications.md"},
		"push.fcm.project_id":                            {Kind: config.KindString, Default: "", Chapter: "13-push-notifications.md"},
		"push.fcm.service_account_ref":                   {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/fcm_service_account.json", Chapter: "13-push-notifications.md"},
		// The agent consumers (M2-07, 04 "Configuration values of this chapter").
		"agent.ack_timeout":          {Kind: config.KindDuration, Default: "30s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"agent.ack_persist_interval": {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"agent.max_inflight_events":  {Kind: config.KindInt, Default: "100", Min: "1", Max: "2147483647", Chapter: "04-sync-protocol.md"},
		// The presence of agents (M2-20).
		"agent.presence_timeout":           {Kind: config.KindDuration, Default: "90s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"agent.heartbeat_persist_interval": {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "03-data-model.md"},
		"billing.mismatch_alert_after":     {Kind: config.KindDuration, Default: "5m", Min: "1ms", Chapter: "10-backend-policy.md"},
		"billing.show_personal_estimates":  {Kind: config.KindBool, Default: "true", Chapter: "10-backend-policy.md"},
		// The statistics queries (M6-05).
		"stats.cache_ttl":                  {Kind: config.KindDuration, Default: "5m", Min: "1s", Max: "1h", Chapter: "17-observability.md"},
		"stats.cache_max_entries":          {Kind: config.KindInt, Default: "1000", Min: "1", Max: "1000000", Chapter: "17-observability.md"},
		"stats.max_range":                  {Kind: config.KindDuration, Default: "400d", Min: "1d", Chapter: "17-observability.md"},
		"stats.time_zone":                  {Kind: config.KindString, Default: "UTC", Chapter: "17-observability.md"},
		"api.stats_max_buckets":            {Kind: config.KindInt, Default: "1000", Min: "1", Max: "100000", Chapter: "06-api.md"},
		"privacy.stats_min_group_size":     {Kind: config.KindInt, Default: "5", Min: "2", Max: "1000", Chapter: "20-privacy.md"},
		"privacy.per_person_stats_enabled": {Kind: config.KindBool, Default: "false", Chapter: "20-privacy.md"},
		"run.step_detail_max_bytes":        {Kind: config.KindInt, Default: "16384", Min: "64", Max: "1048576", Chapter: "03-data-model.md"},
		// M2-08
		"run.queued_max_age": {Kind: config.KindDuration, Default: "48h", Min: "1s", Chapter: "09-agents.md"},
		// M2-09
		"agent.cancel_grace": {Kind: config.KindDuration, Default: "30s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"run.orphan_timeout": {Kind: config.KindDuration, Default: "15m", Min: "1ms", Chapter: "09-agents.md"},
		"run.stall_after":    {Kind: config.KindDuration, Default: "10m", Min: "1ms", Chapter: "03-data-model.md"},
		// M2-13: the byte bounds of the run context (09 4.6 and 21).
		"agent.context_max_bytes":         {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "09-agents.md"},
		"agent.context_message_max_bytes": {Kind: config.KindInt, Default: "32768", Min: "1", Chapter: "09-agents.md"},
		// M2-15
		"component.default_ttl":         {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "03-data-model.md"},
		"component.max_per_message":     {Kind: config.KindInt, Default: "25", Min: "1", Max: "1000", Chapter: "09-agents.md"},
		"component.custom_id_max_chars": {Kind: config.KindInt, Default: "100", Min: "1", Chapter: "09-agents.md"},
		"component.form_max_fields":     {Kind: config.KindInt, Default: "10", Min: "1", Chapter: "09-agents.md"},
		// M2-12
		"agent.session_ref_max_bytes": {Kind: config.KindInt, Default: "1024", Min: "1", Max: "2048", Chapter: "09-agents.md"},
		// M1-64
		"sync.ws_max_frame_bytes":   {Kind: config.KindInt, Default: "1048576", Min: "1024", Max: "16777216", Chapter: "04-sync-protocol.md"},
		"sync.identify_timeout":     {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"sync.heartbeat_interval":   {Kind: config.KindDuration, Default: "30s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"sync.heartbeat_miss_limit": {Kind: config.KindInt, Default: "2", Min: "0", Max: "100", Chapter: "04-sync-protocol.md"},
		"sync.resume_window":        {Kind: config.KindDuration, Default: "5m", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"sync.reauth_lead_time":     {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"sync.outbound_queue_max":   {Kind: config.KindInt, Default: "10000", Min: "1", Max: "1000000", Chapter: "04-sync-protocol.md"},
		"gateway.write_timeout":     {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "02-architecture.md"},
		// M1-66
		"sync.max_focused_threads": {Kind: config.KindInt, Default: "20", Min: "1", Max: "1000", Chapter: "04-sync-protocol.md"},
		"sync.typing_min_interval": {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		"sync.typing_ttl":          {Kind: config.KindDuration, Default: "8s", Min: "1ms", Chapter: "04-sync-protocol.md"},
		// M2-10
		"run.excerpt_max_chars":            {Kind: config.KindInt, Default: "500", Min: "1", Max: "100000", Chapter: "03-data-model.md"},
		"run.max_steps_persisted":          {Kind: config.KindInt, Default: "2000", Min: "1", Max: "100000", Chapter: "03-data-model.md"},
		"sync.run_patch_coalesce_interval": {Kind: config.KindDuration, Default: "250ms", Min: "1ms", Chapter: "04-sync-protocol.md"},
		// M6-13
		"stats.flush_interval":       {Kind: config.KindDuration, Default: "10s", Min: "1s", Chapter: "25-visualization.md"},
		"stats.flush_queue_max":      {Kind: config.KindInt, Default: "20000", Min: "1", Max: "10000000", Chapter: "25-visualization.md"},
		"stats.live_enabled":         {Kind: config.KindBool, Default: "true", Chapter: "25-visualization.md"},
		"stats.live_interval":        {Kind: config.KindDuration, Default: "1s", Min: "100ms", Chapter: "25-visualization.md"},
		"stats.live_max_runs":        {Kind: config.KindInt, Default: "200", Min: "1", Max: "10000", Chapter: "25-visualization.md"},
		"stats.live_max_subscribers": {Kind: config.KindInt, Default: "200", Min: "1", Max: "100000", Chapter: "25-visualization.md"},
		"stats.live_sli_window":      {Kind: config.KindDuration, Default: "5m", Min: "1m", Chapter: "25-visualization.md"},
		// FEAT-topology-agent-tree
		"stats.live_max_tasks":      {Kind: config.KindInt, Default: "25", Min: "0", Max: "500", Chapter: "25-visualization.md"},
		"stats.live_task_max_depth": {Kind: config.KindInt, Default: "4", Min: "1", Max: "16", Chapter: "25-visualization.md"},
		"stats.live_task_keep":      {Kind: config.KindDuration, Default: "2m", Min: "1s", Max: "1h", Chapter: "25-visualization.md"},
		"retention.stats_minutes":   {Kind: config.KindDuration, Default: "90d", Min: "1d", Chapter: "20-privacy.md"},
		// M6-14
		"stats.timeline_max_range":    {Kind: config.KindDuration, Default: "24h", Min: "1m", Max: "31d", Chapter: "25-visualization.md"},
		"stats.timeline_max_runs":     {Kind: config.KindInt, Default: "5000", Min: "1", Max: "100000", Chapter: "25-visualization.md"},
		"stats.slo_warn_budget_ratio": {Kind: config.KindFloat, Default: "0.25", Min: "0", Max: "1", Chapter: "25-visualization.md"},
		"stats.slo_min_events":        {Kind: config.KindInt, Default: "100", Min: "1", Max: "1000000000", Chapter: "25-visualization.md"},
		// M7-09
		"status.check_interval":       {Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: "16-operations.md"},
		"status.degrade_after":        {Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: "16-operations.md"},
		"status.incident_after":       {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "16-operations.md"},
		"status.resolve_after":        {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "16-operations.md"},
		"status.error_ratio_degraded": {Kind: config.KindFloat, Default: "0.05", Min: "0", Max: "1", Chapter: "16-operations.md"},
		"status.error_ratio_outage":   {Kind: config.KindFloat, Default: "0.5", Min: "0", Max: "1", Chapter: "16-operations.md"},
		"status.fanout_lag_degraded":  {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "16-operations.md"},
		"status.public_enabled":       {Kind: config.KindBool, Default: "false", Chapter: "05-actions.md"},
		"status.history_window":       {Kind: config.KindDuration, Default: "7d", Min: "1m", Chapter: "06-api.md"},
		"retention.ops":               {Kind: config.KindDuration, Default: "395d", Min: "1d", Chapter: "20-privacy.md"},
		// M7-06: the privacy requests (20 section 13).
		"privacy.export_ttl":               {Kind: config.KindDuration, Default: "7d", Min: "1h", Chapter: "20-privacy.md"},
		"privacy.export_requires_approval": {Kind: config.KindBool, Default: "false", Chapter: "20-privacy.md"},
		"privacy.account_deletion_content": {Kind: config.KindEnum, Default: "keep_pseudonymized", Enum: []string{"keep_pseudonymized", "delete"}, Chapter: "20-privacy.md"},
		"privacy.request_reminder_lead":    {Kind: config.KindDuration, Default: "7d", Min: "1h", Max: "27d", Chapter: "20-privacy.md"},
		"retention.privacy_job":            {Kind: config.KindDuration, Default: "1095d", Min: "1d", Chapter: "20-privacy.md"},
		// M7-07: the retention of the deletion journal (20 section 13).
		"privacy.deletion_journal_retention": {Kind: config.KindDuration, Default: "400d", Min: "1d", Chapter: "20-privacy.md"},
	}
	// Two keys per class of 05 "Rate-limit classes" (M1-19), with the defaults of 05 section 10.
	for class, def := range map[string][2]string{
		"rl.public_auth": {"30", "10"}, "rl.gateway_connect": {"12", "4"}, "rl.read": {"600", "100"},
		"rl.write": {"120", "30"}, "rl.message": {"60", "20"}, "rl.trigger": {"30", "10"},
		"rl.interaction": {"60", "20"}, "rl.upload": {"300", "60"}, "rl.search": {"60", "20"},
		"rl.typing": {"30", "5"}, "rl.admin": {"60", "20"}, "rl.export": {"2", "2"},
		"rl.agent_runtime": {"1200", "200"}, "rl.webhook_in": {"60", "20"}, "rl.import": {"120", "30"},
		"rl.terminal": {"20", "5"}, "rl.host": {"120", "30"},
	} {
		want["ratelimit."+class+".per_minute"] = config.Key{Kind: config.KindInt, Default: def[0], Min: "1", Max: "1000000", Chapter: "05-actions.md"}
		want["ratelimit."+class+".burst"] = config.Key{Kind: config.KindInt, Default: def[1], Min: "1", Max: "1000000", Chapter: "05-actions.md"}
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
// schema has a row in the table "Configuration values of this chapter" of its chapter with the
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
		case "empty":
			want = ""
		case "no default, required":
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

// chapterDefault returns the cell of the column "Default" in the row of name in the section
// "Configuration values of this chapter" of chapter.
func chapterDefault(t *testing.T, chapter, name string) (string, bool) {
	t.Helper()
	text := readSpec(t, chapter)
	start := strings.Index(text, "Configuration values of this chapter\n")
	if start < 0 {
		t.Fatalf("%s has no section Configuration values of this chapter", chapter)
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
				if c == "Default" {
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

// testPublicURL sets server.public_url, the one key without a default (16 "Configuration values of this chapter").
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

// TestHSTSMaxAgeWholeSeconds holds http.hsts_max_age (19 "Configuration values of this chapter", SEC-TLS-3): a
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

// TestTelemetryDashboardURLIsHTTP (M6-06): telemetry.dashboard_url is empty or an http or https
// URL with a host, because the administration opens it in the system browser; any other value
// stops the start with a problem that names the key and never repeats the value.
func TestTelemetryDashboardURLIsHTTP(t *testing.T) {
	t.Parallel()
	const env = "BOTHY_TELEMETRY__DASHBOARD_URL="
	for _, ok := range []string{"", "https://telemetry.example.com/d/bothy", "http://127.0.0.1:3001/d/x?orgId=1"} {
		if got := load(t, env+ok); got != nil {
			t.Errorf("%s%q: problems %v", env, ok, got)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "ftp://example.com/x", "https://", "telemetry.example.com", "https://user:pw@example.com/"} {
		got := load(t, env+bad)
		if len(got) != 1 || !strings.Contains(got[0].Key+" "+got[0].Message, "telemetry.dashboard_url") {
			t.Errorf("%s%q: problems %v, want one about telemetry.dashboard_url", env, bad, got)
		}
		for _, p := range got {
			if strings.Contains(p.Message, bad) {
				t.Errorf("problem %q repeats the value", p.Message)
			}
		}
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

// TestSearchConfig_StartChecks holds the checks search_page_size and search_headline_words
// (D-M3-14-14): search.page_size_default above search.page_size_max and search.headline_min_words
// at or above search.headline_max_words stop the start with both key names and no value; the
// defaults and a minimum below the maximum pass. test/integration proves with a real ts_headline
// call that PostgreSQL refuses the stopped pairs.
func TestSearchConfig_StartChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		env   []string
		check string
		keys  []string
	}{
		{env: []string{"BOTHY_SEARCH__PAGE_SIZE_DEFAULT=51"}, check: "search_page_size", keys: []string{"search.page_size_default", "search.page_size_max"}},
		{env: []string{"BOTHY_SEARCH__PAGE_SIZE_DEFAULT=5", "BOTHY_SEARCH__PAGE_SIZE_MAX=4"}, check: "search_page_size",
			keys: []string{"search.page_size_default", "search.page_size_max"}},
		{env: []string{"BOTHY_SEARCH__HEADLINE_MIN_WORDS=30"}, check: "search_headline_words", keys: []string{"search.headline_min_words", "search.headline_max_words"}},
		{env: []string{"BOTHY_SEARCH__HEADLINE_MIN_WORDS=31"}, check: "search_headline_words", keys: []string{"search.headline_min_words", "search.headline_max_words"}},
		{env: []string{"BOTHY_SEARCH__PAGE_SIZE_DEFAULT=50", "BOTHY_SEARCH__HEADLINE_MIN_WORDS=29"}},
		{},
	} {
		got := load(t, c.env...)
		if c.check == "" {
			if len(got) != 0 {
				t.Errorf("%v: problems %v", c.env, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Errorf("%v: problems %v, want one of %s", c.env, got, c.check)
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, c.check+":") {
			t.Errorf("%v: problem %q does not come from %s", c.env, msg, c.check)
		}
		for _, k := range c.keys {
			if !strings.Contains(msg, k) {
				t.Errorf("%v: problem %q does not name %s", c.env, msg, k)
			}
		}
		for _, n := range []string{"51", "30", "31"} {
			if strings.Contains(strings.ReplaceAll(msg, "search.", ""), " "+n) {
				t.Errorf("%v: problem %q names a value", c.env, msg)
			}
		}
	}
}

// TestSearchConfig_BoundsFitTheStatements: the search keys that reach int32 parameters of the
// search statements (with the extra look-ahead row of max_candidates and page_size_max, and the
// row bound of the messaging adapter), the ts_headline options or SET LOCAL statement_timeout are
// refused at load above what every search type accepts, and accepted at it.
func TestSearchConfig_BoundsFitTheStatements(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		env string
		key string
		ok  bool
	}{
		{env: "BOTHY_SEARCH__MAX_CANDIDATES=1048575", key: "search.max_candidates", ok: true},
		{env: "BOTHY_SEARCH__MAX_CANDIDATES=1048576", key: "search.max_candidates"},
		{env: "BOTHY_SEARCH__HEADLINE_INPUT_MAX_CHARS=1048575", key: "search.headline_input_max_chars", ok: true},
		{env: "BOTHY_SEARCH__HEADLINE_INPUT_MAX_CHARS=1048576", key: "search.headline_input_max_chars"},
		{env: "BOTHY_SEARCH__PAGE_SIZE_MAX=1048576", key: "search.page_size_max"},
		{env: "BOTHY_SEARCH__PREFIX_MIN_CHARS=2147483648", key: "search.prefix_min_chars"},
		{env: "BOTHY_SEARCH__HEADLINE_MAX_FRAGMENTS=2147483648", key: "search.headline_max_fragments"},
		{env: "BOTHY_SEARCH__STATEMENT_TIMEOUT=24d", key: "search.statement_timeout", ok: true},
		{env: "BOTHY_SEARCH__STATEMENT_TIMEOUT=25d", key: "search.statement_timeout"},
	} {
		got := load(t, c.env)
		if c.ok {
			if len(got) != 0 {
				t.Errorf("%s: problems %v", c.env, got)
			}
			continue
		}
		if len(got) != 1 || !strings.Contains(got[0].String(), c.key) {
			t.Errorf("%s: problems %v, want one naming %s", c.env, got, c.key)
		}
	}
}

// TestSearchExtractConfig_TikaURLUnknown: search.extract.tika_url is planned for V3 and not in the
// schema, so a configuration that sets it is refused as an unknown key (M3-15).
func TestSearchExtractConfig_TikaURLUnknown(t *testing.T) {
	t.Parallel()
	for _, k := range schema(t).Keys() {
		if k.Name == "search.extract.tika_url" {
			t.Fatal("search.extract.tika_url is in the schema")
		}
	}
	got := load(t, "BOTHY_SEARCH__EXTRACT__TIKA_URL=http://127.0.0.1:9998")
	if len(got) != 1 || !strings.Contains(got[0].String(), "BOTHY_SEARCH__EXTRACT__TIKA_URL") {
		t.Fatalf("problems %v, want the unknown variable", got)
	}
}

// TestConfig_PresenceTimeoutAboveHeartbeatInterval: agent.presence_timeout must be greater than
// sync.heartbeat_interval (cross-check presence_timeout_above_heartbeat_interval, M2-20): equal
// values abort the start with a problem naming both keys and no value.
func TestConfig_PresenceTimeoutAboveHeartbeatInterval(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		timeout, interval string
		ok                bool
	}{
		{"90s", "30s", true},
		{"31s", "30s", true},
		{"30s", "30s", false},
		{"20s", "30s", false},
	} {
		got := load(t, "BOTHY_AGENT__PRESENCE_TIMEOUT="+c.timeout, "BOTHY_SYNC__HEARTBEAT_INTERVAL="+c.interval) // nosemgrep: bothy-go-test-gateway-window -- only loads the configuration, no gateway runs
		if (len(got) == 0) != c.ok {
			t.Errorf("timeout %s, interval %s: problems %v, want ok %v", c.timeout, c.interval, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "agent.presence_timeout") || !strings.Contains(msg, "sync.heartbeat_interval") ||
			strings.Contains(msg, " "+c.timeout) {
			t.Errorf("problem %q", msg)
		}
	}
}

// TestConfigCheckBackupSchedules: every schedule of the backup service must be a five-field cron
// expression (cross-check backup_schedules, M7-02); the problem names the key and no value.
func TestConfigCheckBackupSchedules(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"backup.pg.full_schedule", "backup.pg.diff_schedule", "backup.logical.schedule", "backup.objects.schedule", "backup.restore_test.schedule"} {
		env := "BOTHY_" + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
		if got := load(t, env+"=15 4 * * 1-5"); len(got) != 0 {
			t.Errorf("%s = 15 4 * * 1-5: problems %v, want none", key, got)
		}
		got := load(t, env+"=61 99 never")
		if len(got) != 1 {
			t.Errorf("%s = 61 99 never: problems %v, want one", key, got)
			continue
		}
		if msg := got[0].String(); !strings.Contains(msg, key) || strings.Contains(msg, "never") {
			t.Errorf("%s: the problem does not name the key or names the value: %q", key, msg)
		}
	}
}

// TestConfigCheckBackupOffsiteTarget: backup.offsite.target is empty (local backup only) or an
// S3 target ParseOffsiteTarget accepts (cross-check backup_offsite_target, M7-02); the problem
// names the key and no value.
func TestConfigCheckBackupOffsiteTarget(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"", "s3:https://s3.example.com/bothy-backup", "s3:https://192.0.2.10:8333/bothy-backup/instance-1"} {
		if got := load(t, "BOTHY_BACKUP__OFFSITE__TARGET="+target); len(got) != 0 {
			t.Errorf("target %q: problems %v, want none", target, got)
		}
	}
	for _, target := range []string{"https://s3.example.com/bothy-backup", "s3:http://s3.example.com/bothy-backup", "s3:https://s3.example.com/"} {
		got := load(t, "BOTHY_BACKUP__OFFSITE__TARGET="+target)
		if len(got) != 1 {
			t.Errorf("target %q: problems %v, want one", target, got)
			continue
		}
		if msg := got[0].String(); !strings.Contains(msg, "backup.offsite.target") || strings.Contains(msg, "s3.example.com") {
			t.Errorf("target %q: the problem does not name the key or names the value: %q", target, msg)
		}
	}
}

// TestConfigCheck_FormatsEnabled (M2-27): every entry of webhook_in.formats_enabled is native or
// alertmanager and appears once; an unknown value and a duplicate fail the start with the key in
// the message and without the value; the default and an empty list pass.
func TestConfigCheck_FormatsEnabled(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		env string
		ok  bool
	}{
		{"", true},
		{`BOTHY_WEBHOOK_IN__FORMATS_ENABLED=[]`, true},
		{`BOTHY_WEBHOOK_IN__FORMATS_ENABLED=["alertmanager"]`, true},
		{`BOTHY_WEBHOOK_IN__FORMATS_ENABLED=["native", "slackish"]`, false},
		{`BOTHY_WEBHOOK_IN__FORMATS_ENABLED=["native", "native"]`, false},
	} {
		var env []string
		if c.env != "" {
			env = append(env, c.env)
		}
		got := load(t, env...)
		if (len(got) == 0) != c.ok {
			t.Errorf("%s: problems %v, want ok %v", c.env, got, c.ok)
			continue
		}
		for _, p := range got {
			msg := p.Key + " " + p.Message
			if !strings.Contains(msg, "webhook_in.formats_enabled") || strings.Contains(msg, "slackish") {
				t.Errorf("%s: problem %q does not name the key or names the value", c.env, msg)
			}
		}
	}
}

// TestConfig_HeartbeatPersistBelowOrphanTimeout: agent.heartbeat_persist_interval must be below
// run.orphan_timeout (cross-check heartbeat_persist_below_orphan_timeout, M2-09, D-M2-09-21):
// equal or larger values abort the start with a problem naming both keys and no value.
func TestConfig_HeartbeatPersistBelowOrphanTimeout(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		persist, orphan string
		ok              bool
	}{
		{"60s", "15m", true},
		{"14m", "15m", true},
		{"15m", "15m", false},
		{"20m", "15m", false},
	} {
		got := load(t, "BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL="+c.persist, "BOTHY_RUN__ORPHAN_TIMEOUT="+c.orphan)
		if (len(got) == 0) != c.ok {
			t.Errorf("persist %s, orphan %s: problems %v, want ok %v", c.persist, c.orphan, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "agent.heartbeat_persist_interval") || !strings.Contains(msg, "run.orphan_timeout") ||
			strings.Contains(msg, " "+c.persist) {
			t.Errorf("problem %q", msg)
		}
	}
}

// TestUploadBurstHoldsTwoAttachmentBatches holds the sizing of the rl.upload defaults (05
// section 10, addendum of 2026-10-30; FIX-upload-retry-storm): the burst holds the requests of
// two messages with message.max_attachments files of one part each (announce, part, complete),
// and a minute refills at least that much, so one person's batch never waits for the bucket.
func TestUploadBurstHoldsTwoAttachmentBatches(t *testing.T) {
	t.Parallel()
	defaults := map[string]string{}
	for _, k := range schema(t).Keys() {
		defaults[k.Name] = k.Default
	}
	number := func(name string) int {
		t.Helper()
		v, err := strconv.Atoi(defaults[name])
		if err != nil {
			t.Fatalf("%s: default %q: %v", name, defaults[name], err)
		}
		return v
	}
	const requestsPerFile = 3
	batch := number("message.max_attachments") * requestsPerFile
	if burst := number("ratelimit.rl.upload.burst"); burst < 2*batch {
		t.Errorf("ratelimit.rl.upload.burst = %d, want at least %d (two batches)", burst, 2*batch)
	}
	if perMinute := number("ratelimit.rl.upload.per_minute"); perMinute < number("ratelimit.rl.upload.burst") {
		t.Errorf("ratelimit.rl.upload.per_minute = %d below its burst", perMinute)
	}
}
