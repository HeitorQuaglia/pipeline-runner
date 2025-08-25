package artifacts

import (
	"time"
)

type ArtifactType string

const (
	ArtifactTypeBinary    ArtifactType = "binary"
	ArtifactTypeArchive   ArtifactType = "archive"
	ArtifactTypeImage     ArtifactType = "image"
	ArtifactTypeDocument  ArtifactType = "document"
	ArtifactTypeLog       ArtifactType = "log"
	ArtifactTypeReport    ArtifactType = "report"
	ArtifactTypeGeneric   ArtifactType = "generic"
)

type ArtifactStatus string

const (
	ArtifactStatusUploading ArtifactStatus = "uploading"
	ArtifactStatusAvailable ArtifactStatus = "available"
	ArtifactStatusExpired   ArtifactStatus = "expired"
	ArtifactStatusDeleted   ArtifactStatus = "deleted"
	ArtifactStatusCorrupted ArtifactStatus = "corrupted"
)

type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
	StorageTypeGCS   StorageType = "gcs"
	StorageTypeAzure StorageType = "azure"
	StorageTypeFTP   StorageType = "ftp"
	StorageTypeHTTP  StorageType = "http"
)

type Artifact struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	
	Type        ArtifactType      `json:"type"`
	Status      ArtifactStatus    `json:"status"`
	
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum,omitempty"`
	MimeType    string            `json:"mime_type,omitempty"`
	
	Storage     StorageConfig     `json:"storage"`
	
	WorkflowID  string            `json:"workflow_id,omitempty"`
	JobID       string            `json:"job_id,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	
	Labels      map[string]string `json:"labels,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	
	Public      bool              `json:"public"`
	
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	DownloadCount int             `json:"download_count"`
	LastAccessed  *time.Time      `json:"last_accessed,omitempty"`
	
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type StorageConfig struct {
	Type        StorageType       `json:"type"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Bucket      string            `json:"bucket,omitempty"`
	Region      string            `json:"region,omitempty"`
	
	AccessKey   string            `json:"access_key,omitempty"`
	SecretKey   string            `json:"secret_key,omitempty"`
	Token       string            `json:"token,omitempty"`
	
	BasePath    string            `json:"base_path,omitempty"`
	
	Encryption  *EncryptionConfig `json:"encryption,omitempty"`
	Compression bool              `json:"compression,omitempty"`
}

type EncryptionConfig struct {
	Enabled     bool              `json:"enabled"`
	Algorithm   string            `json:"algorithm,omitempty"`
	Key         string            `json:"key,omitempty"`
	KMSKeyID    string            `json:"kms_key_id,omitempty"`
}

type ArtifactRepository struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"`
	
	Storage     StorageConfig     `json:"storage"`
	
	RetentionPolicy *RetentionPolicy `json:"retention_policy,omitempty"`
	AccessPolicy    *AccessPolicy    `json:"access_policy,omitempty"`
	
	Public      bool              `json:"public"`
	
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	ArtifactCount int64           `json:"artifact_count"`
	TotalSize     int64           `json:"total_size"`
}

type RetentionPolicy struct {
	MaxAge      *time.Duration    `json:"max_age,omitempty"`
	MaxCount    int               `json:"max_count,omitempty"`
	MaxSize     int64             `json:"max_size,omitempty"`
	
	Rules       []RetentionRule   `json:"rules,omitempty"`
}

type RetentionRule struct {
	Condition   string            `json:"condition"`
	Action      string            `json:"action"`
	Value       string            `json:"value,omitempty"`
}

type AccessPolicy struct {
	Public      bool              `json:"public"`
	Users       []string          `json:"users,omitempty"`
	Groups      []string          `json:"groups,omitempty"`
	Permissions []Permission      `json:"permissions,omitempty"`
}

type Permission struct {
	Subject     string            `json:"subject"`
	SubjectType string            `json:"subject_type"`
	Actions     []string          `json:"actions"`
}

type ArtifactVersion struct {
	ID          string            `json:"id"`
	ArtifactID  string            `json:"artifact_id"`
	Version     string            `json:"version"`
	
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum,omitempty"`
	
	Changes     string            `json:"changes,omitempty"`
	
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	
	DownloadCount int             `json:"download_count"`
}

type ArtifactShare struct {
	ID          string            `json:"id"`
	ArtifactID  string            `json:"artifact_id"`
	Token       string            `json:"token"`
	
	Password    string            `json:"password,omitempty"`
	MaxDownloads int              `json:"max_downloads,omitempty"`
	
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	
	DownloadCount int             `json:"download_count"`
	LastAccessed  *time.Time      `json:"last_accessed,omitempty"`
}

type ArtifactSearch struct {
	Query       string            `json:"query,omitempty"`
	Type        ArtifactType      `json:"type,omitempty"`
	Status      ArtifactStatus    `json:"status,omitempty"`
	
	Labels      map[string]string `json:"labels,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	
	WorkflowID  string            `json:"workflow_id,omitempty"`
	JobID       string            `json:"job_id,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	
	MinSize     int64             `json:"min_size,omitempty"`
	MaxSize     int64             `json:"max_size,omitempty"`
	
	CreatedAfter  *time.Time      `json:"created_after,omitempty"`
	CreatedBefore *time.Time      `json:"created_before,omitempty"`
	
	SortBy      string            `json:"sort_by,omitempty"`
	SortOrder   string            `json:"sort_order,omitempty"`
	
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
}