package main

import (
	"fmt"

	"github.com/arreyder/pprof-mcp/internal/datadog"
	"github.com/arreyder/pprof-mcp/internal/profiles"
)

type ProfileHandle struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Bytes  int64  `json:"bytes"`
}

type RegisteredBundle struct {
	Handles      []ProfileHandle
	HandleByType map[string]string
	PathByType   map[string]string
}

var profileRegistry = profiles.NewRegistry()

func registerBundleHandles(result datadog.DownloadResult) (RegisteredBundle, error) {
	bundle := RegisteredBundle{
		Handles:      make([]ProfileHandle, 0, len(result.Files)),
		HandleByType: make(map[string]string, len(result.Files)),
		PathByType:   make(map[string]string, len(result.Files)),
	}
	for _, file := range result.Files {
		handle, err := profileRegistry.Register(profiles.Metadata{
			Service:   result.Service,
			Env:       result.Env,
			Type:      file.Type,
			Timestamp: result.Timestamp,
			ProfileID: result.ProfileID,
			EventID:   result.EventID,
			Path:      file.Path,
			Bytes:     file.Bytes,
		})
		if err != nil {
			return RegisteredBundle{}, err
		}
		bundle.Handles = append(bundle.Handles, ProfileHandle{
			Type:   file.Type,
			Handle: handle,
			Bytes:  file.Bytes,
		})
		bundle.HandleByType[file.Type] = handle
		bundle.PathByType[file.Type] = file.Path
	}
	return bundle, nil
}

func resolveHandlePath(baseDir, value string) (string, error) {
	meta, ok := profileRegistry.Resolve(value)
	if !ok {
		return "", fmt.Errorf("unknown profile handle %q", value)
	}
	return sanitizePath(baseDir, meta.Path)
}

func isHandle(value string) bool {
	return profiles.IsHandle(value)
}

func resolveHandleMeta(value string) (profiles.Metadata, error) {
	meta, ok := profileRegistry.Resolve(value)
	if !ok {
		return profiles.Metadata{}, fmt.Errorf("unknown profile handle %q", value)
	}
	return meta, nil
}

func findBundleMetas(handle string) ([]profiles.Metadata, error) {
	meta, err := resolveHandleMeta(handle)
	if err != nil {
		return nil, err
	}
	all := profileRegistry.All()
	matched := []profiles.Metadata{}
	for _, item := range all {
		if sameBundle(meta, item) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		matched = append(matched, meta)
	}
	return matched, nil
}

func sameBundle(a, b profiles.Metadata) bool {
	if a.ProfileID != "" && b.ProfileID != "" && a.EventID != "" && b.EventID != "" {
		return a.ProfileID == b.ProfileID && a.EventID == b.EventID
	}
	if a.Timestamp != "" && b.Timestamp != "" && a.Service != "" && b.Service != "" {
		return a.Timestamp == b.Timestamp && a.Service == b.Service && a.Env == b.Env
	}
	return false
}
