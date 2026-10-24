// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"io"
	"time"

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
}

// moduleFiles is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleFiles = "files"

// filesWiring wires the files module: its start step (6f'), the health monitor of the virus
// scanner (worker, all; M3-04) and the job files.scan; the start step binds the port
// files.Attachments that messaging and runs received earlier (M3-07).
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
	f, err := NewFiles(cfg, s.values.String(keyStorageDriver), store, pool, a.UoW(), az, FilesWrites{Events: sy.Appender()},
		c.Service, s.deps.Clock, s.logger())
	if err != nil {
		if cl, ok := store.(io.Closer); ok {
			_ = cl.Close()
		}
		return err
	}
	if s.api() {
		// Downloads, tickets and transcripts are served by the roles with routes; the ticket key
		// must exist and be long enough before such a role starts (D-M3-05-15).
		if err := s.enableDownloads(ctx, f); err != nil {
			_ = f.Close()
			return err
		}
	}
	if s.worker() {
		if err := s.enableFilesJobs(f, a.Recorder()); err != nil {
			_ = f.Close()
			return err
		}
	}
	s.attachments.Bind(f.Service)
	s.importFiles.Bind(f.Service.ImportFiles())
	s.mu.Lock()
	s.files = f
	id, st := s.identity, s.settings
	s.mu.Unlock()
	f.BindProfileImages(id, st)
	return nil
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

// enableFilesJobs builds the use cases of files.scan (M3-04) and files.thumbnail (M3-06) in a
// role that runs jobs.
func (s *server) enableFilesJobs(f *Files, audit auditing.Recorder) error {
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
	return f.EnableImages(imgCfg, s.logger(), s.deps.FilesImageHooks)
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
