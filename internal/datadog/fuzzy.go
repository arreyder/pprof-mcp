package datadog

import (
	"fmt"
	"sort"
	"strings"
)

// FuzzyMatch represents a service match with scoring information.
type FuzzyMatch struct {
	Service      string   `json:"service"`
	Environments []string `json:"environments,omitempty"`
	Score        float64  `json:"score"`      // 0-1, higher is better
	MatchType    string   `json:"match_type"` // "exact", "prefix", "contains", "normalized", "similar"
}

// FuzzySearchServices searches for services matching the query.
// Returns matches sorted by score (highest first).
func FuzzySearchServices(query string, services []ServiceInfo) []FuzzyMatch {
	if query == "" || len(services) == 0 {
		return nil
	}

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)
	queryNormalized := normalizeServiceName(query)

	var matches []FuzzyMatch

	for _, svc := range services {
		nameLower := strings.ToLower(svc.Name)
		nameNormalized := normalizeServiceName(svc.Name)

		var score float64
		var matchType string

		// 1. Exact match
		if svc.Name == query {
			score = 1.0
			matchType = "exact"
		} else if nameLower == queryLower {
			// 2. Case-insensitive exact
			score = 0.95
			matchType = "exact"
		} else if strings.HasPrefix(nameLower, queryLower) {
			// 3. Prefix match
			score = 0.9
			matchType = "prefix"
		} else if nameNormalized == queryNormalized {
			// 4. Normalized exact match (temporal_worker == temporal-worker == temporalworker)
			score = 0.85
			matchType = "normalized"
		} else if strings.HasPrefix(nameNormalized, queryNormalized) {
			// 5. Normalized prefix match
			score = 0.8
			matchType = "normalized"
		} else if strings.Contains(nameLower, queryLower) {
			// 6. Contains
			score = 0.7
			matchType = "contains"
		} else if strings.Contains(nameNormalized, queryNormalized) {
			// 7. Normalized contains
			score = 0.65
			matchType = "contains"
		} else {
			// 8. Levenshtein distance for typo tolerance
			distance := levenshteinDistance(queryNormalized, nameNormalized)
			maxLen := max(len(queryNormalized), len(nameNormalized))

			// Calculate max allowed edits based on length
			maxEdits := 2
			if len(queryNormalized) >= 8 {
				maxEdits = 3
			}

			if distance <= maxEdits && maxLen > 0 {
				// Score based on similarity ratio
				similarity := 1.0 - float64(distance)/float64(maxLen)
				score = similarity * 0.6 // Cap at 0.6 for fuzzy matches
				matchType = "similar"
			}
		}

		if score > 0 {
			matches = append(matches, FuzzyMatch{
				Service:      svc.Name,
				Environments: svc.Environments,
				Score:        score,
				MatchType:    matchType,
			})
		}
	}

	// Sort by score descending, then by name ascending for stability
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Service < matches[j].Service
	})

	return matches
}

// normalizeServiceName removes separators and converts to lowercase for comparison.
func normalizeServiceName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, " ", "")
	return name
}

// levenshteinDistance calculates the edit distance between two strings.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	rows := len(s1) + 1
	cols := len(s2) + 1
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[rows-1][cols-1]
}

// FormatFuzzyMatchesTable formats matches as a readable table.
func FormatFuzzyMatchesTable(matches []FuzzyMatch) string {
	if len(matches) == 0 {
		return "No matching services found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-30s  %-12s  %-6s  %s\n", "SERVICE", "MATCH TYPE", "SCORE", "ENVIRONMENTS"))
	sb.WriteString(strings.Repeat("-", 90) + "\n")

	for _, m := range matches {
		envs := strings.Join(m.Environments, ", ")
		if len(envs) > 35 {
			envs = envs[:32] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-30s  %-12s  %.2f    %s\n", m.Service, m.MatchType, m.Score, envs))
	}

	return sb.String()
}

// TopMatches returns the top N matches.
func TopMatches(matches []FuzzyMatch, n int) []FuzzyMatch {
	if len(matches) <= n {
		return matches
	}
	return matches[:n]
}

// BestMatch returns the highest-scoring match, or nil if no matches.
func BestMatch(matches []FuzzyMatch) *FuzzyMatch {
	if len(matches) == 0 {
		return nil
	}
	return &matches[0]
}
