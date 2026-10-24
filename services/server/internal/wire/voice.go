// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// VoiceConfig binds the keys of voice messages and transcripts (M3-09): voice.max_duration,
// voice.transcription_mode, voice.transcription_max_attempts and message.max_content_bytes. The
// error names the key whose binding failed, never a value.
func VoiceConfig(v *config.Values) (filesapp.VoiceConfig, error) {
	var c filesapp.VoiceConfig
	if err := config.Bind(v, &c); err != nil {
		return filesapp.VoiceConfig{}, err
	}
	return c, nil
}

// VoiceSwitch binds the outgoing port filesapp.VoiceSwitch to the feature flag voice.enabled of
// instance (M1-47, M3-09 design note 15): files needs no edge to instance, only the composition
// root knows where flags live.
type VoiceSwitch struct {
	// Flags are the feature flags of instance.
	Flags instance.Flags
}

// Compile-time check: VoiceSwitch is the port.
var _ filesapp.VoiceSwitch = VoiceSwitch{}

// Enabled implements filesapp.VoiceSwitch.
func (v VoiceSwitch) Enabled(ctx context.Context, principalID ids.ID, principalKind string) (bool, error) {
	return v.Flags.Enabled(ctx, instance.FlagSubject{PrincipalID: principalID, Kind: principalKind}, instance.FlagVoiceEnabled)
}

// VoicePrincipals binds filesapp.Principals to the directory of identity: the kind from
// Principals, the locale from User for a person.
type VoicePrincipals struct {
	// Directory is identity.Directory.
	Directory identity.Directory
}

// Compile-time check: VoicePrincipals is the port.
var _ filesapp.Principals = VoicePrincipals{}

// Principal implements filesapp.Principals. Errors: not_found for an unknown id.
func (p VoicePrincipals) Principal(ctx context.Context, id ids.ID) (string, string, error) {
	found, err := p.Directory.Principals(ctx, []ids.ID{id})
	if err != nil {
		return "", "", err
	}
	pv, ok := found[id]
	if !ok {
		return "", "", errs.New(errs.CodeNotFound, "")
	}
	kind := string(pv.Kind)
	if pv.Kind != actor.KindUser {
		return kind, "", nil
	}
	u, err := p.Directory.User(ctx, id)
	if err != nil {
		return "", "", err
	}
	return kind, u.Locale, nil
}

// enableVoice adds voice messages and transcripts to the files module in every role (M3-09): the
// voice check of createMessage and createThread, requestTranscription and submitTranscript need
// the role api, the job files.transcribe the roles worker and all.
func (s *server) enableVoice(f *Files) error {
	cfg, err := VoiceConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, st := s.identity, s.settings
	s.mu.Unlock()
	if id == nil || id.Directory == nil || st == nil || st.Admin == nil {
		return errors.New("wire: voice messages need the directory of identity and the feature flags of instance")
	}
	d := filesapp.VoiceDeps{Switch: VoiceSwitch{Flags: st.Admin}, Queue: f.TranscribeQueue(), Principals: VoicePrincipals{Directory: id.Directory}}
	return f.Service.EnableVoice(cfg, d)
}

// enableTranscription builds the use case of files.transcribe in a role that runs jobs (M3-09):
// the adapter openai_compatible of FilesTranscriber (nil outside mode service).
func (s *server) enableTranscription(ctx context.Context, f *Files) error {
	cfg, err := VoiceConfig(s.values)
	if err != nil {
		return err
	}
	tr, err := FilesTranscriber(ctx, s.values, s.role, TranscriptionDeps{Secrets: s.sp, Egress: s.egressDeps(), Logger: s.logger()})
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, st := s.identity, s.settings
	s.mu.Unlock()
	return f.EnableTranscription(cfg, tr, VoiceSwitch{Flags: st.Admin}, VoicePrincipals{Directory: id.Directory}, s.logger())
}
