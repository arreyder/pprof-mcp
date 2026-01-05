package textutil

import "strings"

type TruncateStrategy string

const (
	StrategyHead     TruncateStrategy = "head"
	StrategyTail     TruncateStrategy = "tail"
	StrategyHeadTail TruncateStrategy = "head_tail"
)

type TruncateOptions struct {
	MaxLines int
	MaxBytes int
	Strategy TruncateStrategy
}

type TruncateMeta struct {
	TotalLines      int    `json:"total_lines"`
	TotalBytes      int    `json:"total_bytes"`
	Truncated       bool   `json:"truncated"`
	TruncatedReason string `json:"truncated_reason,omitempty"`
	Strategy        string `json:"strategy,omitempty"`
}

type TruncateResult struct {
	Text string
	Meta TruncateMeta
}

const truncateMarker = "... (truncated) ..."

func TruncateText(raw string, opts TruncateOptions) TruncateResult {
	// Apply line/byte limits using the chosen strategy and return explicit metadata.
	strategy := normalizeStrategy(opts.Strategy)
	result := TruncateResult{
		Text: raw,
		Meta: TruncateMeta{
			TotalLines: CountLines(raw),
			TotalBytes: len(raw),
			Strategy:   string(strategy),
		},
	}

	trimmed := raw
	reasons := []string{}

	if opts.MaxLines > 0 && result.Meta.TotalLines > opts.MaxLines {
		trimmed = truncateByLines(trimmed, opts.MaxLines, strategy)
		reasons = append(reasons, "max_lines")
	}
	if opts.MaxBytes > 0 && len(trimmed) > opts.MaxBytes {
		trimmed = truncateByBytes(trimmed, opts.MaxBytes, strategy)
		reasons = append(reasons, "max_bytes")
	}

	if len(reasons) > 0 {
		result.Text = trimmed
		result.Meta.Truncated = true
		result.Meta.TruncatedReason = strings.Join(reasons, ",")
	}

	return result
}

func CountLines(raw string) int {
	if raw == "" {
		return 0
	}
	count := strings.Count(raw, "\n")
	if strings.HasSuffix(raw, "\n") {
		return count
	}
	return count + 1
}

func normalizeStrategy(strategy TruncateStrategy) TruncateStrategy {
	switch strategy {
	case StrategyHead, StrategyTail, StrategyHeadTail:
		return strategy
	default:
		return StrategyHead
	}
}

func truncateByLines(raw string, maxLines int, strategy TruncateStrategy) string {
	if maxLines <= 0 {
		return raw
	}
	lines := splitLines(raw)
	if len(lines) <= maxLines {
		return raw
	}
	switch strategy {
	case StrategyTail:
		return strings.Join(lines[len(lines)-maxLines:], "\n")
	case StrategyHeadTail:
		if maxLines <= 1 {
			return lines[0]
		}
		headCount := maxLines / 2
		tailCount := maxLines - headCount - 1
		if tailCount <= 0 {
			return strings.Join(lines[:maxLines], "\n")
		}
		output := make([]string, 0, maxLines)
		output = append(output, lines[:headCount]...)
		output = append(output, truncateMarker)
		output = append(output, lines[len(lines)-tailCount:]...)
		return strings.Join(output, "\n")
	default:
		return strings.Join(lines[:maxLines], "\n")
	}
}

func truncateByBytes(raw string, maxBytes int, strategy TruncateStrategy) string {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		return raw
	}
	switch strategy {
	case StrategyTail:
		return raw[len(raw)-maxBytes:]
	case StrategyHeadTail:
		if maxBytes <= len(truncateMarker) {
			return raw[:maxBytes]
		}
		headCount := (maxBytes - len(truncateMarker)) / 2
		tailCount := maxBytes - len(truncateMarker) - headCount
		if tailCount <= 0 {
			return raw[:maxBytes]
		}
		return raw[:headCount] + truncateMarker + raw[len(raw)-tailCount:]
	default:
		return raw[:maxBytes]
	}
}

func splitLines(raw string) []string {
	trimmed := strings.TrimSuffix(raw, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
