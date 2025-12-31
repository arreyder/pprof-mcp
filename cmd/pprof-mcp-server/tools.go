package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
	"github.com/conductorone/mcp-go-sdk/mcp/server"
)

// ToolDefinition combines a tool's schema with its handler.
type ToolDefinition struct {
	Schema  schema.Tool
	Handler server.ToolHandlerFunc
}

// SchemaToolsProvider implements ToolsProvider with full schema support.
type SchemaToolsProvider struct {
	tools map[string]ToolDefinition
	mu    sync.RWMutex
}

// NewSchemaToolsProvider creates a new provider with schema support.
func NewSchemaToolsProvider() *SchemaToolsProvider {
	return &SchemaToolsProvider{
		tools: make(map[string]ToolDefinition),
	}
}

// AddTool registers a tool with its full schema.
func (p *SchemaToolsProvider) AddTool(def ToolDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools[def.Schema.Name] = def
}

// List returns all registered tools with their full schemas.
func (p *SchemaToolsProvider) List(ctx context.Context) ([]schema.Tool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tools := make([]schema.Tool, 0, len(p.tools))
	for _, def := range p.tools {
		tools = append(tools, def.Schema)
	}
	return tools, nil
}

// Call executes a registered tool.
func (p *SchemaToolsProvider) Call(ctx context.Context, name string, args map[string]any) (*schema.CallToolResult, error) {
	p.mu.RLock()
	def, exists := p.tools[name]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	result, err := def.Handler(ctx, args)
	if err != nil {
		return &schema.CallToolResult{
			IsError: true,
			Content: []schema.PromptContentUnion{
				schema.NewPromptContentFromText(fmt.Sprintf("Error: %v", err)),
			},
		}, nil
	}

	var content []schema.PromptContentUnion
	switch v := result.(type) {
	case string:
		content = []schema.PromptContentUnion{
			schema.NewPromptContentFromText(v),
		}
	case []schema.PromptContentUnion:
		content = v
	case schema.PromptContentUnion:
		content = []schema.PromptContentUnion{v}
	default:
		content = []schema.PromptContentUnion{
			schema.NewPromptContentFromText(fmt.Sprintf("%v", v)),
		}
	}

	return &schema.CallToolResult{
		Content: content,
	}, nil
}

// Helper to create JSON schema property
func prop(typ, desc string) json.RawMessage {
	p := map[string]string{"type": typ, "description": desc}
	data, _ := json.Marshal(p)
	return data
}

// Helper to create enum property
func enumProp(typ, desc string, values []string) json.RawMessage {
	p := map[string]any{"type": typ, "description": desc, "enum": values}
	data, _ := json.Marshal(p)
	return data
}

// Helper to create array property
func arrayProp(itemType, desc string) json.RawMessage {
	p := map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]string{"type": itemType},
	}
	data, _ := json.Marshal(p)
	return data
}

// ToolSchemas returns all tool definitions.
func ToolSchemas() []ToolDefinition {
	return []ToolDefinition{
		{
			Schema: schema.Tool{
				Name:        "profiles.download_latest_bundle",
				Description: "Download the latest profiling bundle from Datadog for a service. Returns paths to downloaded pprof files.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":    prop("string", "The service name to download profiles for (required)"),
						"env":        prop("string", "The environment (e.g., prod, staging) (required)"),
						"out_dir":    prop("string", "Output directory for downloaded profiles (required)"),
						"hours":      prop("integer", "Number of hours to look back for profiles (default: 72)"),
						"dd_site":    prop("string", "Datadog site (e.g., datadoghq.com, datadoghq.eu)"),
						"profile_id": prop("string", "Specific profile ID to download"),
						"event_id":   prop("string", "Specific event ID to download"),
					},
					Required: []string{"service", "env", "out_dir"},
				},
			},
			Handler: downloadTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.top",
				Description: "Run pprof top command to show the top functions by CPU/memory usage. Returns structured data with function names, flat/cumulative values, and percentages.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show (default: 10)"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTopTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.peek",
				Description: "Run pprof peek command to show callers and callees of functions matching a regex pattern.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
						"binary":  prop("string", "Path to the binary for symbol resolution"),
						"regex":   prop("string", "Regex pattern to match function names (required)"),
					},
					Required: []string{"profile", "regex"},
				},
			},
			Handler: pprofPeekTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.list",
				Description: "Run pprof list command to show annotated source code for a function with line-level profiling data.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"function":     prop("string", "Function name or regex to list source for (required)"),
						"repo_root":    prop("string", "Repository root path for source file resolution"),
						"trim_path":    prop("string", "Path prefix to trim from source file paths"),
						"source_paths": arrayProp("string", "Additional source paths for vendored or external dependencies"),
					},
					Required: []string{"profile", "function"},
				},
			},
			Handler: pprofListTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.traces_head",
				Description: "Run pprof traces command and return the first N lines of stack traces.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
						"binary":  prop("string", "Path to the binary for symbol resolution"),
						"lines":   prop("integer", "Maximum number of lines to return (default: 200)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTracesTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.diff_top",
				Description: "Compare two pprof profiles and show the difference in top functions. Useful for identifying performance regressions or improvements.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"before":       prop("string", "Path to the baseline pprof profile (required)"),
						"after":        prop("string", "Path to the comparison pprof profile (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
					},
					Required: []string{"before", "after"},
				},
			},
			Handler: pprofDiffTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.meta",
				Description: "Extract metadata from a pprof profile including sample types, duration, drop frames, and comments.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofMetaTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.storylines",
				Description: "Analyze profile to find the top N 'storylines' - hot code paths in your repository. Shows the most expensive execution paths with source-level detail.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":     prop("string", "Path to the pprof profile file (required)"),
						"n":           prop("integer", "Number of storylines to return (default: 4)"),
						"focus":       prop("string", "Regex to focus on specific functions"),
						"ignore":      prop("string", "Regex to ignore specific functions"),
						"repo_prefix": arrayProp("string", "Repository path prefixes to identify your code (e.g., github.com/myorg/myrepo)"),
						"repo_root":   prop("string", "Local repository root path for source file resolution"),
						"trim_path":   prop("string", "Path prefix to trim from source file paths"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofStorylinesTool,
		},
		{
			Schema: schema.Tool{
				Name:        "datadog.profiles.list",
				Description: "List available profiling data from Datadog for a service. Returns profile metadata including timestamps and IDs.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service": prop("string", "The service name to list profiles for (required)"),
						"env":     prop("string", "The environment (e.g., prod, staging) (required)"),
						"from":    prop("string", "Start time (RFC3339 or relative like '-1h')"),
						"to":      prop("string", "End time (RFC3339 or relative)"),
						"hours":   prop("integer", "Number of hours to look back (default: 72, ignored if from/to set)"),
						"limit":   prop("integer", "Maximum number of profiles to return (default: 50)"),
						"site":    prop("string", "Datadog site (e.g., datadoghq.com)"),
					},
					Required: []string{"service", "env"},
				},
			},
			Handler: datadogProfilesListTool,
		},
		{
			Schema: schema.Tool{
				Name:        "datadog.profiles.pick",
				Description: "Pick a specific profile from Datadog based on a selection strategy. Useful for selecting profiles at specific times or with specific characteristics.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":   prop("string", "The service name (required)"),
						"env":       prop("string", "The environment (required)"),
						"from":      prop("string", "Start time (RFC3339 or relative)"),
						"to":        prop("string", "End time (RFC3339 or relative)"),
						"hours":     prop("integer", "Number of hours to look back (default: 72)"),
						"limit":     prop("integer", "Maximum profiles to consider (default: 50)"),
						"site":      prop("string", "Datadog site"),
						"strategy":  enumProp("string", "Selection strategy", []string{"latest", "oldest", "closest", "index"}),
						"target_ts": prop("string", "Target timestamp for 'closest' strategy (RFC3339)"),
						"index":     prop("integer", "Index for 'index' strategy (0-based)"),
					},
					Required: []string{"service", "env"},
				},
			},
			Handler: datadogProfilesPickTool,
		},
		{
			Schema: schema.Tool{
				Name:        "repo.services.discover",
				Description: "Discover services in a repository by scanning for common patterns like Dockerfiles, go.mod, package.json, etc.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"repo_root": prop("string", "Root directory of the repository to scan (default: current directory)"),
					},
					Required: []string{},
				},
			},
			Handler: repoServicesTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.tags",
				Description: "Show or filter profile data by tags/labels. Can list available tags or filter results by tag values (e.g., tenant_id, connector_id).",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"tag_focus":    prop("string", "Regex to focus on samples with matching tag values"),
						"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
						"tag_show":     prop("string", "Show values for a specific tag key (runs -tags mode)"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTagsTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.flamegraph",
				Description: "Generate a flamegraph SVG visualization from a pprof profile. Useful for visualizing hot code paths.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"output_path":  prop("string", "Path to write the SVG file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"tag_focus":    prop("string", "Regex to focus on samples with matching tag values"),
						"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "output_path"},
				},
			},
			Handler: pprofFlamegraphTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.callgraph",
				Description: "Generate a call graph visualization from a pprof profile in DOT, SVG, or PNG format.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"output_path":  prop("string", "Path to write the output file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"format":       enumProp("string", "Output format", []string{"dot", "svg", "png"}),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"edge_frac":    prop("number", "Hide edges below this fraction (0.0-1.0)"),
						"node_frac":    prop("number", "Hide nodes below this fraction (0.0-1.0)"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "output_path"},
				},
			},
			Handler: pprofCallgraphTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.focus_paths",
				Description: "Show all call paths that lead to a specific function. Useful for understanding how a hot function is reached.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"function":     prop("string", "Target function name or regex to find paths to (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of paths to show"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "function"},
				},
			},
			Handler: pprofFocusPathsTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.merge",
				Description: "Merge multiple pprof profiles into a single aggregated profile. Useful for combining profiles from different time periods or instances.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profiles":    arrayProp("string", "List of profile paths to merge (required, minimum 2)"),
						"output_path": prop("string", "Path to write the merged profile (required)"),
						"binary":      prop("string", "Path to the binary for symbol resolution"),
					},
					Required: []string{"profiles", "output_path"},
				},
			},
			Handler: pprofMergeTool,
		},
	}
}
