// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The keys of the server transcript (M3-09 design note 16) that the cross-check
// voice_service_url and FilesTranscriber read.
const (
	keyVoiceTranscriptionMode = "voice.transcription_mode"
	keyVoiceTranscriptionURL  = "voice.transcription_url"
	keyVoiceAPIKeyRef         = "voice.transcription_api_key_ref"
)

// The values of voice.transcription_mode (12 8.2): voiceModeService is the one that sends audio
// to the transcription service.
const (
	voiceModeOff     = "off"
	voiceModeAgent   = "agent"
	voiceModeService = "service"
)

// egressPurposeTranscription labels the egress client of the transcription service in logs.
const egressPurposeTranscription = "transcription"

// transcriptionValues are the keys of the adapter openai_compatible as config.Bind fills them.
type transcriptionValues struct {
	Mode            string        `config:"voice.transcription_mode"`
	URL             string        `config:"voice.transcription_url"`
	APIKeyRef       string        `config:"voice.transcription_api_key_ref"`
	Model           string        `config:"voice.transcription_model"`
	Timeout         time.Duration `config:"voice.transcription_timeout"`
	MaxContentBytes int64         `config:"message.max_content_bytes"`
}

// TranscriptionDeps are the collaborators of the transcription adapter from the process.
type TranscriptionDeps struct {
	// Secrets resolves voice.transcription_api_key_ref: the provider of the process
	// (NewSecretsProvider).
	Secrets transcription.SecretResolver
	// Egress are the resolver, roots, logger and metrics of the egress client, the same as every
	// other egress consumer of the process gets.
	Egress egress.Deps
	// Logger receives the Warn line of a failed call; nil discards it.
	Logger *slog.Logger
}

// FilesTranscriber builds the port app.Transcriber of the files module (M3-09 design note 11):
// the adapter openai_compatible over one egress client with purpose transcription whose host
// policy allows exactly the scheme, host and port of voice.transcription_url, no redirects, the
// time limit voice.transcription_timeout and the body limit egress.max_response_bytes, with
// voice.transcription_api_key_ref, voice.transcription_model and message.max_content_bytes.
//
// It returns nil, nil when voice.transcription_mode is not service or the role does not run the
// worker (only the job files.transcribe calls the port), so the role api never needs the key.
// In mode service it resolves the key reference once, so a key that cannot be read stops the
// start (fail fast); the adapter resolves it again on every call (secrets.cache_ttl), so a
// rotation needs no restart.
//
// Errors name keys, never values: a schema without the keys (config.Bind), egress.* that
// EgressConfig refuses, a URL or deps the adapter refuses, a key that cannot be resolved.
func FilesTranscriber(ctx context.Context, v *config.Values, role Role, d TranscriptionDeps) (filesapp.Transcriber, error) {
	if role != RoleWorker && role != RoleAll {
		return nil, nil
	}
	var raw transcriptionValues
	if err := config.Bind(v, &raw); err != nil {
		return nil, err
	}
	if raw.Mode != voiceModeService {
		return nil, nil
	}
	eg, err := EgressConfig(v)
	if err != nil {
		return nil, err
	}
	hosts, err := transcription.HostPolicy(raw.URL)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if raw.APIKeyRef != "" {
		if d.Secrets == nil {
			return nil, fmt.Errorf("config: %s: no secret provider to resolve it", keyVoiceAPIKeyRef)
		}
		if _, _, err := d.Secrets.Resolve(ctx, raw.APIKeyRef); err != nil {
			return nil, fmt.Errorf("config: %s: %s", keyVoiceAPIKeyRef, secrets.ErrorCode(err))
		}
	}
	client, err := egress.New(eg, egress.Options{Purpose: egressPurposeTranscription, Hosts: hosts,
		Redirects: egress.RedirectNever, Timeout: raw.Timeout}, d.Egress)
	if err != nil {
		return nil, fmt.Errorf("config: %s: egress client: %w", keyVoiceTranscriptionURL, err)
	}
	a, err := transcription.NewOpenAICompatible(transcription.Config{URL: raw.URL, APIKeyRef: raw.APIKeyRef, Model: raw.Model,
		MaxTextBytes: int(raw.MaxContentBytes)}, transcription.Deps{Client: client, Secrets: d.Secrets, Logger: d.Logger})
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return a, nil
}

// voiceServiceURL is the cross-check voice_service_url (M3-09 design note 16): mode service
// requires voice.transcription_url to be an absolute URL with scheme https or http, a host, no
// user info and no fragment (transcription.ValidateURL). Other modes never send audio, so the
// URL is not checked there. The error names the keys, never the value.
func voiceServiceURL(v *config.Values) error {
	if v.String(keyVoiceTranscriptionMode) != voiceModeService {
		return nil
	}
	if err := transcription.ValidateURL(v.String(keyVoiceTranscriptionURL)); err != nil {
		return fmt.Errorf("%s = %s: %w", keyVoiceTranscriptionMode, voiceModeService, err)
	}
	return nil
}
