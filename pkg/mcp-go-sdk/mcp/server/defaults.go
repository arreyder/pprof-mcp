package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
)

// DefaultToolsProvider is a simple map-based implementation of ToolsProvider
type DefaultToolsProvider struct {
	tools map[string]ToolHandlerFunc
	mu    sync.RWMutex
}

// NewDefaultToolsProvider creates a new default tools provider
func NewDefaultToolsProvider() *DefaultToolsProvider {
	return &DefaultToolsProvider{
		tools: make(map[string]ToolHandlerFunc),
	}
}

// AddTool registers a tool with the provider
func (p *DefaultToolsProvider) AddTool(name string, handler ToolHandlerFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools[name] = handler
}

// List returns all registered tools
func (p *DefaultToolsProvider) List(ctx context.Context) ([]schema.Tool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tools := make([]schema.Tool, 0, len(p.tools))
	for name := range p.tools {
		tools = append(tools, schema.Tool{
			Name: name,
			InputSchema: schema.ToolInputSchema{
				Type: "object",
			},
		})
	}
	return tools, nil
}

// Call executes a registered tool
func (p *DefaultToolsProvider) Call(ctx context.Context, name string, args map[string]any) (*schema.CallToolResult, error) {
	p.mu.RLock()
	handler, exists := p.tools[name]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: tool %q not found", ErrMethodNotFound, name)
	}

	// Call the handler
	result, err := handler(ctx, args)
	if err != nil {
		// Return error as a result to be visible to the LLM
		return &schema.CallToolResult{
			IsError: true,
			Content: []schema.PromptContentUnion{
				schema.NewPromptContentFromText(fmt.Sprintf("Error: %v", err)),
			},
		}, nil
	}

	// Convert scalar result to text content
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
		// For other types, convert to string
		content = []schema.PromptContentUnion{
			schema.NewPromptContentFromText(fmt.Sprintf("%v", v)),
		}
	}

	return &schema.CallToolResult{
		Content: content,
	}, nil
}
