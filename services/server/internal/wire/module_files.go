// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
)

// filesState is the part of the server state the files module fills.
type filesState struct {
	// files is the files module (M3-03): the upload sessions over the object store, built in the
	// files step after chat.
	files *Files
	// attachments is the port files.Attachments that messaging and runs receive in the chat step
	// and the files step binds to the files module (M3-07).
	attachments LazyAttachments
	// transcripts is the port files.Transcripts that runs receives in the chat step and the
	// files step binds to the files module (M3-09).
	transcripts LazyTranscripts
}

// moduleFiles is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleFiles = "files"

// filesWiring wires the files module: its start step (6f'), the health monitor of the virus
// scanner (worker, all; M3-04), the jobs files.scan and files.thumbnail, files.purge_objects
// and the periodic files.cleanup_pending (M3-08), files.extract_text and the periodic
// files.extract_text_refresh (M3-15), and files.transcribe (M3-09); the start step binds the
// ports files.Attachments (M3-07) and files.Transcripts (M3-09) that messaging and runs received
// earlier.
func filesWiring() moduleWiring {
	return moduleWiring{
		module: moduleFiles,
		parts: []part{
			{order: 1400, step: moduleFiles, start: (*server).startFiles},
			{order: 2500, step: "scan_health", when: (*server).worker, start: (*server).startScanHealth},
		},
		handlers: func(s *server, h *Handlers) { s.files.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			f := s.files
			s.mu.Unlock()
			m.FilesScan = f.ScanRunner()
			m.FilesImages, m.FilesImageTimeout = f.ImageRunner(), s.imageTimeout()
			m.FilesPurge, m.FilesCleanup, m.FilesGCInterval = f.PurgeRunner(), f.CleanupRunner(), s.values.Duration(keyFileGCInterval)
			m.FilesExtractText, m.FilesExtractRefresh = f.ExtractRunners()
			m.FilesTranscribe, m.FilesTranscribeAttempts = f.TranscribeRunner(), int(s.values.Int("voice.transcription_max_attempts"))
			m.FilesExtractInterval = s.values.Duration("search.extract.refresh_interval")
			return nil
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			f := s.files
			s.mu.Unlock()
			f.BindJobs(ins)
		},
	}
}

// startFiles is start step 6f' (M3-03): it opens the object store of file.storage.driver
// (FilesObjectStore: the driver fs checks file.fs.root, the driver s3 its keys, secrets and egress)
// and builds the files module over the pool, the unit of work of the audit step, the Authorizer,
// the appender of sync and the structure of chat. A root that fs.Open refuses ends the start.
func (s *server) startFiles(ctx context.Context) error {
	cfg, err := FilesConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c := s.pool, s.audit, s.authorizer, s.sync, s.chat
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || c == nil {
		return errors.New("wire: the files step needs audit, the Authorizer, sync and chat")
	}
	store, err := FilesObjectStore(ctx, s.values, FilesDeps{Secrets: s.sp, Egress: s.egressDeps()})
	if err != nil {
		return err
	}
	f, err := NewFiles(cfg, s.values.String(keyStorageDriver), store, pool, a.UoW(), az, FilesWrites{Events: sy.Appender(), Redactor: sy.Redactor()},
		c.Service, s.deps.Clock, s.logger())
	if err != nil {
		if cl, ok := store.(io.Closer); ok {
			_ = cl.Close()
		}
		return err
	}
	if err := s.enableFilesParts(ctx, f, a.Recorder(), sy.DraftFiles()); err != nil {
		_ = f.Close()
		return err
	}
	s.attachments.Bind(f.Service)
	s.transcripts.Bind(f.Service)
	s.importFiles.Bind(f.Service.ImportFiles())
	s.mu.Lock()
	s.files = f
	id, st := s.identity, s.settings
	s.mu.Unlock()
	f.BindProfileImages(id, st)
	return nil
}

// enableFilesParts adds the parts of the files module by role: downloads, tickets and
// transcripts in the roles with routes (the ticket key must exist and be long enough before such a
// role starts, D-M3-05-15), voice messages in every role (M3-09), the job use cases in the roles
// that run jobs.
func (s *server) enableFilesParts(ctx context.Context, f *Files, audit auditing.Recorder, drafts files.DraftFileSource) error {
	if s.api() {
		if err := s.enableDownloads(ctx, f); err != nil {
			return err
		}
	}
	if err := s.enableVoice(f); err != nil {
		return err
	}
	if !s.worker() {
		return nil
	}
	if err := s.enableFilesJobs(f, audit, drafts); err != nil {
		return err
	}
	return s.enableTranscription(ctx, f)
}

// enableDownloads binds the download keys and the ticket key source and adds the downloads to f.
func (s *server) enableDownloads(ctx context.Context, f *Files) error {
	cfg, err := DownloadConfig(s.values)
	if err != nil {
		return err
	}
	keys, err := NewTicketKeys(ctx, s.sp, s.values.String(keyTicketKeyRef))
	if err != nil {
		return err
	}
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()
	return f.EnableDownloads(cfg, id, keys)
}

// enableFilesJobs builds the use cases of files.extract_text and files.extract_text_refresh
// (M3-15), files.scan (M3-04), files.thumbnail (M3-06), files.purge_objects and
// files.cleanup_pending (M3-08, with the draft source of sync) in a role that runs jobs; the text
// extraction comes first, because the scan pass and the image job take its trigger.
func (s *server) enableFilesJobs(f *Files, audit auditing.Recorder, drafts files.DraftFileSource) error {
	extCfg, xCfg, err := ExtractConfigs(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()
	if id == nil || id.Directory == nil {
		return errors.New("wire: the text extraction needs the directory of identity")
	}
	if err := f.EnableExtraction(extCfg, xCfg, id.Directory, s.deps.Clock, s.logger()); err != nil {
		return err
	}
	scanCfg, clamdCfg, err := ScanConfigs(s.values)
	if err != nil {
		return err
	}
	if err := f.EnableScan(scanCfg, clamdCfg, audit, s.deps.Clock, s.logger()); err != nil {
		return err
	}
	imgCfg, err := ImageConfig(s.values)
	if err != nil {
		return err
	}
	if err := f.EnableImages(imgCfg, s.logger(), s.deps.FilesImageHooks); err != nil {
		return err
	}
	cleanCfg, err := CleanupConfig(s.values)
	if err != nil {
		return err
	}
	return f.EnableCleanup(cleanCfg, drafts, s.deps.Clock, s.logger())
}

// imageTimeout is file.image.processing_timeout, the River timeout of files.thumbnail in every
// role (M3-06).
func (s *server) imageTimeout() time.Duration {
	return s.values.Duration("file.image.processing_timeout")
}

// startScanHealth starts the health monitor of the virus scanner in the roles worker and all
// (M3-04, 12 5.3): a loop that pings clamd every file.scan.health_interval until the stop step
// scan_health ends it. It never changes readiness.
func (s *server) startScanHealth(context.Context) error {
	s.mu.Lock()
	f := s.files
	s.mu.Unlock()
	if f == nil || f.Health == nil {
		return errors.New("wire: the scan health step needs the scan pass of the files step")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.Health.Run(ctx)
	}()
	s.runner.OnStop(lifecycle.StopStep{Name: "scan_health", Phase: lifecycle.PhaseJobs, Run: func(stop context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stop.Done():
			return stop.Err()
		}
	}})
	return nil
}
