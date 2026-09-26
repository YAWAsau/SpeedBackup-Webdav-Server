package sbserver

const (
	ServerVersion  = "0.3.15-webdav16"
	ProtocolMajor  = 1
	ProtocolMinor  = 0
	ManifestSchema = "speedbackup.server.manifest.v1"
	SessionSchema  = "speedbackup.server.session.v1"
)

type ProtocolVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

type ManifestEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Kind   string `json:"kind,omitempty"`
	AppID  string `json:"app_id,omitempty"`
}

type GenerationManifest struct {
	Schema         string          `json:"schema"`
	Generation     uint64          `json:"generation"`
	DeviceID       string          `json:"device_id"`
	ProfileID      string          `json:"profile_id"`
	SessionID      string          `json:"session_id"`
	CommitID       string          `json:"commit_id"`
	CreatedUnix    int64           `json:"created_unix"`
	BaseGeneration *uint64         `json:"base_generation,omitempty"`
	ManifestSHA256 string          `json:"manifest_sha256"`
	Entries        []ManifestEntry `json:"entries"`
}

type UploadedObject struct {
	SHA256       string `json:"sha256"`
	ExpectedSize int64  `json:"expected_size"`
	ReceivedSize int64  `json:"received_size"`
	Complete     bool   `json:"complete"`
	UpdatedUnix  int64  `json:"updated_unix"`
}

type SessionMeta struct {
	Schema              string                    `json:"schema"`
	ID                  string                    `json:"id"`
	DeviceID            string                    `json:"device_id"`
	ProfileID           string                    `json:"profile_id"`
	BaseGeneration      *uint64                   `json:"base_generation,omitempty"`
	State               string                    `json:"state"`
	CreatedUnix         int64                     `json:"created_unix"`
	UpdatedUnix         int64                     `json:"updated_unix"`
	Uploads             map[string]UploadedObject `json:"uploads"`
	CommittedGeneration *uint64                   `json:"committed_generation,omitempty"`
	CommitID            string                    `json:"commit_id,omitempty"`
}

type AppDetailsAuditRequest struct {
	SeedOK      bool     `json:"seed_ok"`
	StageApps   []string `json:"stage_apps"`
	SeedApps    []string `json:"seed_apps"`
	PayloadApps []string `json:"payload_apps"`
}

type AppDetailsAuditResponse struct {
	Schema                   string   `json:"schema"`
	Allowed                  bool     `json:"allowed"`
	SeedlessRepair           bool     `json:"seedless_repair"`
	SeedlessTainted          bool     `json:"seedless_tainted"`
	MissingStagePayloadApps  []string `json:"missing_stage_payload_apps"`
	MissingSeedApps          []string `json:"missing_seed_apps"`
	IgnoredRemotePayloadApps []string `json:"ignored_remote_payload_apps"`
}

type ServerConfig struct {
	Schema      string `json:"schema"`
	TokenSHA256 string `json:"token_sha256"`
	CreatedUnix int64  `json:"created_unix"`
}
type CreateSessionRequest struct {
	DeviceID       string  `json:"device_id"`
	ProfileID      string  `json:"profile_id"`
	BaseGeneration *uint64 `json:"base_generation"`
}
type CreateSessionResponse struct {
	Session           SessionMeta `json:"session"`
	CurrentGeneration *uint64     `json:"current_generation,omitempty"`
}
type CommitRequest struct {
	CommitID       string          `json:"commit_id"`
	BaseGeneration *uint64         `json:"base_generation"`
	Entries        []ManifestEntry `json:"entries"`
}
type CommitResponse struct {
	Committed        bool   `json:"committed"`
	IdempotentReplay bool   `json:"idempotent_replay"`
	Generation       uint64 `json:"generation"`
	ManifestSHA256   string `json:"manifest_sha256"`
}
type ManifestDiffRequest struct {
	DeviceID  string          `json:"device_id"`
	ProfileID string          `json:"profile_id"`
	Entries   []ManifestEntry `json:"entries"`
}
type ManifestDiffResponse struct {
	BaseGeneration       *uint64  `json:"base_generation,omitempty"`
	UploadRequiredSHA256 []string `json:"upload_required_sha256"`
	UnchangedPaths       []string `json:"unchanged_paths"`
	ChangedPaths         []string `json:"changed_paths"`
	RemoteOnlyPaths      []string `json:"remote_only_paths"`
}
type CleanupRequest struct {
	DryRun  *bool  `json:"dry_run,omitempty"`
	Confirm string `json:"confirm,omitempty"`
}
type CleanupResponse struct {
	DryRun         bool     `json:"dry_run"`
	OrphanObjects  uint64   `json:"orphan_objects"`
	OrphanBytes    uint64   `json:"orphan_bytes"`
	DeletedObjects uint64   `json:"deleted_objects"`
	DeletedBytes   uint64   `json:"deleted_bytes"`
	SampleSHA256   []string `json:"sample_sha256"`
}
type AuditEvent struct {
	Unix    int64  `json:"unix"`
	Event   string `json:"event"`
	Message string `json:"message"`
	Details any    `json:"details"`
}
type StorageStats struct {
	ObjectCount     uint64 `json:"object_count"`
	ObjectBytes     uint64 `json:"object_bytes"`
	SessionCount    uint64 `json:"session_count"`
	ProfileCount    uint64 `json:"profile_count"`
	GenerationCount uint64 `json:"generation_count"`
}
type ProfileSummary struct {
	DeviceID          string  `json:"device_id"`
	ProfileID         string  `json:"profile_id"`
	CurrentGeneration *uint64 `json:"current_generation,omitempty"`
	EntryCount        uint64  `json:"entry_count"`
	LogicalBytes      uint64  `json:"logical_bytes"`
}
