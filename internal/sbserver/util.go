package sbserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

func nowUnix() int64              { return time.Now().Unix() }
func SHA256Bytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ValidateSHA256(s string) error {
	if len(s) != 64 {
		return errors.New("sha256 must be 64 hex chars")
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return errors.New("invalid sha256")
		}
	}
	return nil
}
func ValidateIdentifier(s, name string) error {
	if s == "" || len(s) > 96 {
		return fmt.Errorf("invalid %s", name)
	}
	if s == "." || s == ".." {
		return fmt.Errorf("invalid %s", name)
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("invalid %s", name)
		}
	}
	return nil
}
func ValidateLogicalPath(p string) error {
	if p == "" || len(p) > 4096 || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, "//") {
		return errors.New("invalid logical path")
	}
	if path.Clean(p) != p || p == "." {
		return errors.New("logical path is not canonical")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("unsafe logical path")
		}
	}
	return nil
}
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sb1_" + hex.EncodeToString(b), nil
}

func AuditAppDetails(req AppDetailsAuditRequest) AppDetailsAuditResponse {
	mk := func(in []string) map[string]struct{} {
		m := map[string]struct{}{}
		for _, s := range in {
			if s != "" {
				m[s] = struct{}{}
			}
		}
		return m
	}
	stage, seed, payload := mk(req.StageApps), mk(req.SeedApps), mk(req.PayloadApps)
	scope := map[string]struct{}{}
	for s := range stage {
		scope[s] = struct{}{}
	}
	if req.SeedOK {
		for s := range seed {
			scope[s] = struct{}{}
		}
	}
	missingStage := []string{}
	for s := range stage {
		if _, ok := payload[s]; !ok {
			missingStage = append(missingStage, s)
		}
	}
	missingSeed := []string{}
	if req.SeedOK {
		for s := range seed {
			if _, ok := stage[s]; !ok {
				missingSeed = append(missingSeed, s)
			}
		}
	}
	ignored := []string{}
	for s := range payload {
		if _, ok := scope[s]; !ok {
			ignored = append(ignored, s)
		}
	}
	sort.Strings(missingStage)
	sort.Strings(missingSeed)
	sort.Strings(ignored)
	repair := !req.SeedOK && len(stage) > 0
	return AppDetailsAuditResponse{Schema: "speedbackup.server.appdetails_audit.v1", Allowed: len(missingStage) == 0 && (!req.SeedOK || len(missingSeed) == 0) && (req.SeedOK || len(stage) > 0), SeedlessRepair: repair, SeedlessTainted: repair && len(ignored) > 0, MissingStagePayloadApps: missingStage, MissingSeedApps: missingSeed, IgnoredRemotePayloadApps: ignored}
}
