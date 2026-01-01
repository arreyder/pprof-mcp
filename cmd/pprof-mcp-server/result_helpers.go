package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolOutput struct {
	Text       string
	Structured any
}

func TextResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

func ErrorResult(err error, hint string) *mcp.CallToolResult {
	payload := buildErrorPayload(err, hint)
	msg := payload["message"].(string)
	if details, ok := payload["details"].(map[string]any); ok {
		if expected, ok := details["expected"].(string); ok && expected != "" {
			msg += "\nExpected: " + expected
		}
		if received, ok := details["received"].(string); ok && received != "" {
			msg += "\nReceived: " + received
		}
		if hintValue, ok := details["hint"].(string); ok && hintValue != "" {
			msg += "\nHint: " + hintValue
		}
	}
	msg = strings.TrimSpace(msg)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		StructuredContent: map[string]any{
			"error": payload,
		},
	}
}

func formatUnexpectedResult(value any) *mcp.CallToolResult {
	return TextResult(fmt.Sprintf("%v", value))
}

func buildErrorPayload(err error, hint string) map[string]any {
	code := "INTERNAL"
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = "CANCELED"
	} else if errors.Is(err, os.ErrNotExist) {
		code = "NOT_FOUND"
	} else if _, ok := err.(*ValidationError); ok {
		code = "INVALID_ARGUMENT"
	}

	message := strings.TrimSpace(err.Error())
	details := map[string]any{}
	if v, ok := err.(*ValidationError); ok {
		details["field"] = v.Field
		if v.Expected != "" {
			details["expected"] = v.Expected
		}
		if v.Received != "" {
			details["received"] = v.Received
		}
		if v.Hint != "" {
			details["hint"] = v.Hint
		} else if hint != "" {
			details["hint"] = hint
		}
	} else if hint != "" {
		details["hint"] = hint
	}

	payload := map[string]any{
		"message": message,
		"code":    code,
	}
	if len(details) > 0 {
		payload["details"] = details
	}
	return payload
}
