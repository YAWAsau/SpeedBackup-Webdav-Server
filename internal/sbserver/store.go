package sbserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Store struct {
	Root        string
	diagnostics *diagnosticLog
	metaMu      sync.Mutex
	commitMu    sync.Mutex
}

func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{"sessions", "manifests", "objects", "audit", "config", "logs"} {
		if err := os.MkdirAll(filepath.Join(abs, ".speedbackup-server", d), 0700); err != nil {
			return nil, err
		}
	}
	return &Store{Root: abs, diagnostics: &diagnosticLog{filename: filepath.Join(abs, ".speedbackup-server", "logs", "server.jsonl"), limit: diagnosticFileLimit}}, nil
}
func (s *Store) internal(parts ...string) string {
	a := append([]string{s.Root, ".speedbackup-server"}, parts...)
	return filepath.Join(a...)
}
func (s *Store) sessionDir(id string) string     { return s.internal("sessions", id) }
func (s *Store) sessionMetaDir(id string) string { return filepath.Join(s.sessionDir(id), "meta") }
func (s *Store) sessionObjectsDir(id string) string {
	return filepath.Join(s.sessionDir(id), "objects")
}
func (s *Store) partPath(id, hash string) string {
	return filepath.Join(s.sessionObjectsDir(id), strings.ToLower(hash)+".part")
}
func (s *Store) readyPath(id, hash string) string {
	return filepath.Join(s.sessionObjectsDir(id), strings.ToLower(hash)+".ready")
}
func (s *Store) objectPath(hash string) string {
	h := strings.ToLower(hash)
	return s.internal("objects", h[:2], h[2:4], h)
}

func writeImmutableJSON(path string, v any) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(parent, ".speedbackup-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	committed := false
	defer func() {
		_ = f.Close()
		if !committed {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Destination is immutable and must not already exist. Rename therefore
	// needs no replace-existing semantics and is consistent on Windows/Linux.
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	committed = true
	// Best-effort directory sync for durable name publication. Some Windows
	// filesystems do not support syncing a directory handle; file fsync above
	// remains mandatory and errors there are never ignored.
	if d, err := os.Open(parent); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
func nextVersion(dir string) (uint64, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return 0, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var max uint64
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(e.Name(), ".json"), 10, 64)
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1, nil
}
func latestVersionFile(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var max uint64
	var found bool
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(e.Name(), ".json"), 10, 64)
		if err == nil && (!found || n > max) {
			max = n
			found = true
		}
	}
	if !found {
		return "", os.ErrNotExist
	}
	return filepath.Join(dir, fmt.Sprintf("%020d.json", max)), nil
}
func (s *Store) SaveSession(m SessionMeta) error {
	if err := ValidateIdentifier(m.ID, "session_id"); err != nil {
		return err
	}
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	dir := s.sessionMetaDir(m.ID)
	n, err := nextVersion(dir)
	if err != nil {
		return err
	}
	return writeImmutableJSON(filepath.Join(dir, fmt.Sprintf("%020d.json", n)), m)
}
func (s *Store) LoadSession(id string) (SessionMeta, error) {
	var m SessionMeta
	if err := ValidateIdentifier(id, "session_id"); err != nil {
		return m, err
	}
	p, err := latestVersionFile(s.sessionMetaDir(id))
	if err != nil {
		return m, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
func NormalizeEntries(in []ManifestEntry) ([]ManifestEntry, error) {
	out := append([]ManifestEntry(nil), in...)
	seen := map[string]struct{}{}
	for i := range out {
		e := &out[i]
		if err := ValidateLogicalPath(e.Path); err != nil {
			return nil, err
		}
		if e.Size < 0 {
			return nil, errors.New("negative size")
		}
		if err := ValidateSHA256(e.SHA256); err != nil {
			return nil, err
		}
		e.SHA256 = strings.ToLower(e.SHA256)
		if _, ok := seen[e.Path]; ok {
			return nil, fmt.Errorf("duplicate manifest path: %s", e.Path)
		}
		seen[e.Path] = struct{}{}
		if len(e.Kind) > 64 {
			return nil, errors.New("invalid kind")
		}
		if len(e.AppID) > 512 {
			return nil, errors.New("invalid app_id")
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func ManifestDigest(entries []ManifestEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return SHA256Bytes(b), nil
}
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Store) generationDir(device, profile string) string {
	return s.internal("manifests", device, profile, "generations")
}
func (s *Store) generationPath(device, profile string, g uint64) string {
	return filepath.Join(s.generationDir(device, profile), fmt.Sprintf("%020d.json", g))
}
func (s *Store) CurrentGenerationNumber(device, profile string) (*uint64, error) {
	if err := ValidateIdentifier(device, "device_id"); err != nil {
		return nil, err
	}
	if err := ValidateIdentifier(profile, "profile_id"); err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(s.generationDir(device, profile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var max uint64
	found := false
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(e.Name(), ".json"), 10, 64)
		if err == nil && (!found || n > max) {
			max = n
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	return &max, nil
}
func (s *Store) LoadGeneration(device, profile string, g uint64) (GenerationManifest, error) {
	var m GenerationManifest
	b, err := os.ReadFile(s.generationPath(device, profile, g))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
func (s *Store) LoadCurrentGeneration(device, profile string) (*GenerationManifest, error) {
	g, err := s.CurrentGenerationNumber(device, profile)
	if err != nil || g == nil {
		return nil, err
	}
	m, err := s.LoadGeneration(device, profile, *g)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *Store) PublishManifest(m GenerationManifest) error {
	return writeImmutableJSON(s.generationPath(m.DeviceID, m.ProfileID, m.Generation), m)
}
func (s *Store) ObjectExistsWithSize(hash string, size int64) (bool, error) {
	if err := ValidateSHA256(hash); err != nil {
		return false, err
	}
	st, err := os.Stat(s.objectPath(hash))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return st.Mode().IsRegular() && st.Size() == size, nil
}
func (s *Store) PromoteObject(sessionID, hash string, size int64) error {
	dst := s.objectPath(hash)
	if ok, err := s.ObjectExistsWithSize(hash, size); err != nil {
		return err
	} else if ok {
		return nil
	}
	src := s.readyPath(sessionID, hash)
	st, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("object %s is not uploaded: %w", hash, err)
	}
	if st.Size() != size {
		return fmt.Errorf("uploaded object size mismatch for %s", hash)
	}
	got, err := fileSHA256(src)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, hash) {
		return fmt.Errorf("uploaded object sha256 mismatch for %s", hash)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		if ok, e2 := s.ObjectExistsWithSize(hash, size); e2 == nil && ok {
			_ = os.Remove(src)
			return nil
		}
		return err
	}
	f, err := os.Open(dst)
	if err == nil {
		_ = f.Sync()
		_ = f.Close()
	}
	return nil
}
func BuildManifest(g uint64, device, profile, session, commit string, base *uint64, entries []ManifestEntry) (GenerationManifest, error) {
	norm, err := NormalizeEntries(entries)
	if err != nil {
		return GenerationManifest{}, err
	}
	dig, err := ManifestDigest(norm)
	if err != nil {
		return GenerationManifest{}, err
	}
	return GenerationManifest{Schema: ManifestSchema, Generation: g, DeviceID: device, ProfileID: profile, SessionID: session, CommitID: commit, CreatedUnix: nowUnix(), BaseGeneration: base, ManifestSHA256: dig, Entries: norm}, nil
}
func (s *Store) ListSessions() ([]SessionMeta, error) {
	root := s.internal("sessions")
	ents, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []SessionMeta{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []SessionMeta{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		m, err := s.LoadSession(e.Name())
		if err == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedUnix > out[j].CreatedUnix })
	return out, nil
}
func (s *Store) ListProfiles() ([]ProfileSummary, error) {
	root := s.internal("manifests")
	devs, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []ProfileSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []ProfileSummary{}
	for _, d := range devs {
		if !d.IsDir() {
			continue
		}
		ps, err := os.ReadDir(filepath.Join(root, d.Name()))
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if !p.IsDir() {
				continue
			}
			m, err := s.LoadCurrentGeneration(d.Name(), p.Name())
			if err != nil {
				return nil, err
			}
			sum := ProfileSummary{DeviceID: d.Name(), ProfileID: p.Name()}
			if m != nil {
				g := m.Generation
				sum.CurrentGeneration = &g
				sum.EntryCount = uint64(len(m.Entries))
				for _, e := range m.Entries {
					sum.LogicalBytes += uint64(e.Size)
				}
			}
			out = append(out, sum)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DeviceID == out[j].DeviceID {
			return out[i].ProfileID < out[j].ProfileID
		}
		return out[i].DeviceID < out[j].DeviceID
	})
	return out, nil
}
func (s *Store) StorageStats() (StorageStats, error) {
	var st StorageStats
	sessions, err := s.ListSessions()
	if err != nil {
		return st, err
	}
	st.SessionCount = uint64(len(sessions))
	profiles, err := s.ListProfiles()
	if err != nil {
		return st, err
	}
	st.ProfileCount = uint64(len(profiles))
	for _, p := range profiles {
		dir := s.generationDir(p.DeviceID, p.ProfileID)
		ents, e := os.ReadDir(dir)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return st, e
		}
		for _, x := range ents {
			if !x.IsDir() && strings.HasSuffix(x.Name(), ".json") {
				st.GenerationCount++
			}
		}
	}
	objroot := s.internal("objects")
	err = filepath.WalkDir(objroot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() {
			info, e := d.Info()
			if e != nil {
				return e
			}
			st.ObjectCount++
			st.ObjectBytes += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return st, err
	}
	return st, nil
}
func (s *Store) AppendEvent(ev AuditEvent) (err error) {
	defer func() {
		if err != nil && s.diagnostics != nil {
			s.diagnostics.mu.Lock()
			s.diagnostics.writeErrors++
			s.diagnostics.lastError = "audit: " + err.Error()
			s.diagnostics.mu.Unlock()
			s.diagnostics.append(map[string]any{"kind": "audit_write_error", "event": ev.Event, "message": err.Error()})
		}
	}()
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	p := s.internal("audit", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	if err := rotateLog(p, diagnosticFileLimit, 0); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
func (s *Store) TailEvents(limit int) ([]AuditEvent, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	out := []AuditEvent{}
	for i := 0; i < diagnosticCopies && len(out) < limit; i++ {
		name := s.internal("audit", "events.jsonl")
		if i > 0 {
			name += fmt.Sprintf(".%d", i)
		}
		f, err := os.Open(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		offset, err := auditTailOffset(f, limit-len(out))
		if err != nil {
			f.Close()
			return nil, err
		}
		if _, err = f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(f, diagnosticFileLimit))
		f.Close()
		if err != nil {
			return nil, err
		}
		chunk := []AuditEvent{}
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			var ev AuditEvent
			if json.Unmarshal([]byte(line), &ev) == nil {
				chunk = append(chunk, ev)
			}
		}
		out = append(chunk, out...)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}
func (s *Store) ConfigDir() string { return s.internal("config") }
func (s *Store) SaveConfig(c ServerConfig) error {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	n, err := nextVersion(s.ConfigDir())
	if err != nil {
		return err
	}
	return writeImmutableJSON(filepath.Join(s.ConfigDir(), fmt.Sprintf("%020d.json", n)), c)
}
func (s *Store) LoadConfig() (ServerConfig, error) {
	var c ServerConfig
	p, err := latestVersionFile(s.ConfigDir())
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}
func (s *Store) LoadOrInitConfig() (ServerConfig, string, error) {
	c, err := s.LoadConfig()
	if err == nil {
		return c, "", nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return c, "", err
	}
	tok, err := GenerateToken()
	if err != nil {
		return c, "", err
	}
	c = ServerConfig{Schema: "speedbackup.server.config.v1", TokenSHA256: SHA256Bytes([]byte(tok)), CreatedUnix: nowUnix()}
	if err = s.SaveConfig(c); err != nil {
		return c, "", err
	}
	return c, tok, nil
}
func (s *Store) CleanupObjects(dry bool) (CleanupResponse, error) {
	s.commitMu.Lock()
	defer s.commitMu.Unlock()
	refs := map[string]struct{}{}
	root := s.internal("manifests")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var m GenerationManifest
		if e = json.Unmarshal(b, &m); e != nil {
			return fmt.Errorf("refusing cleanup: invalid manifest %s: %w", p, e)
		}
		if m.Schema != ManifestSchema {
			return fmt.Errorf("refusing cleanup: unexpected manifest schema in %s", p)
		}
		for _, x := range m.Entries {
			if e = ValidateSHA256(x.SHA256); e != nil {
				return fmt.Errorf("refusing cleanup: invalid sha256 in %s", p)
			}
			refs[strings.ToLower(x.SHA256)] = struct{}{}
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return CleanupResponse{}, err
	}
	resp := CleanupResponse{DryRun: dry, SampleSHA256: []string{}}
	objroot := s.internal("objects")
	err = filepath.WalkDir(objroot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if ValidateSHA256(name) != nil {
			return nil
		}
		if _, ok := refs[strings.ToLower(name)]; ok {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		resp.OrphanObjects++
		resp.OrphanBytes += uint64(info.Size())
		if len(resp.SampleSHA256) < 20 {
			resp.SampleSHA256 = append(resp.SampleSHA256, name)
		}
		if !dry {
			if e = os.Remove(p); e != nil {
				return e
			}
			resp.DeletedObjects++
			resp.DeletedBytes += uint64(info.Size())
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return resp, err
	}
	return resp, nil
}
