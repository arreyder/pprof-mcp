package main

import (
	"strings"

	"github.com/arreyder/pprof-mcp/internal/textutil"
)

func applyTextLimits(raw string, baseMeta *textutil.TruncateMeta, maxLines, maxBytes int) (string, textutil.TruncateMeta) {
	result := textutil.TruncateText(raw, textutil.TruncateOptions{
		MaxLines: maxLines,
		MaxBytes: maxBytes,
		Strategy: textutil.StrategyHead,
	})
	meta := result.Meta
	if baseMeta != nil {
		meta = mergeTruncateMeta(*baseMeta, result.Meta)
	}
	return result.Text, meta
}

func mergeTruncateMeta(base, extra textutil.TruncateMeta) textutil.TruncateMeta {
	merged := base
	if merged.TotalBytes == 0 {
		merged.TotalBytes = extra.TotalBytes
	}
	if merged.TotalLines == 0 {
		merged.TotalLines = extra.TotalLines
	}
	merged.Truncated = base.Truncated || extra.Truncated
	merged.TruncatedReason = mergeReasons(base.TruncatedReason, extra.TruncatedReason)
	if !merged.Truncated {
		merged.TruncatedReason = ""
	}
	return merged
}

func mergeReasons(reasons ...string) string {
	seen := map[string]struct{}{}
	ordered := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		for _, item := range strings.Split(reason, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			ordered = append(ordered, item)
		}
	}
	return strings.Join(ordered, ",")
}

func addStderr(payload map[string]any, stderr string, meta textutil.TruncateMeta) {
	if strings.TrimSpace(stderr) == "" && !meta.Truncated {
		return
	}
	payload["stderr"] = stderr
	payload["stderr_meta"] = meta
}
