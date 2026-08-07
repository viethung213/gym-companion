package adk

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// promptDumpDirEnv names a directory that receives one JSON file per model
// call, capturing the exact Contents handed to Gemini. Unset disables it.
//
// Traces answer "how many tokens"; only this answers "which tokens". ADK's
// generate_content span records usage counts and no prompt text, so an agent
// whose history filter admits more than intended is invisible in Jaeger.
const promptDumpDirEnv = "COACH_PROMPT_DUMP_DIR"

// promptDump is one captured model request.
type promptDump struct {
	Agent string `json:"agent"`
	Model string `json:"model"`

	// Branch is what ADK filters conversation history by, so a surprising
	// Contents is often explained by it. (IsolationScope, the other filter,
	// is not reachable from a callback context.)
	Branch string `json:"branch"`

	CapturedAt        string           `json:"captured_at"`
	SystemInstruction string           `json:"system_instruction,omitempty"`
	ToolNames         []string         `json:"tool_names,omitempty"`
	ContentCount      int              `json:"content_count"`
	Contents          []*genai.Content `json:"contents"`
}

// beforeModelCallbacks appends the opt-in dumper to an agent's safety
// callbacks, leaving them untouched when dumping is disabled.
//
// agentName is passed in rather than read from the context: a callback
// context does not implement Agent().
func beforeModelCallbacks(agentName string, safety ...llmagent.BeforeModelCallback) []llmagent.BeforeModelCallback {
	dir := strings.TrimSpace(os.Getenv(promptDumpDirEnv))
	if dir == "" {
		return safety
	}
	return append(safety, newPromptDumper(dir, agentName))
}

// newPromptDumper returns a callback writing each request to dir. It always
// reports (nil, nil): a debugging aid must never suppress a model call, so
// every failure is logged and swallowed.
func newPromptDumper(dir, agentName string) llmagent.BeforeModelCallback {
	var seq atomic.Int64

	return func(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
		if req == nil {
			return nil, nil
		}

		dump := promptDump{
			Agent:        agentName,
			Model:        req.Model,
			Branch:       ctx.Branch(),
			CapturedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			ContentCount: len(req.Contents),
			Contents:     req.Contents,
		}
		if req.Config != nil {
			dump.SystemInstruction = contentText(req.Config.SystemInstruction)
			dump.ToolNames = declaredToolNames(req.Config.Tools)
		}

		if err := writeDump(dir, agentName, seq.Add(1), dump); err != nil {
			log.Printf("coaching: prompt dump skipped: %v", err)
		}

		return nil, nil
	}
}

// writeDump names files so a lexical sort replays the call order.
func writeDump(dir, agentName string, n int64, dump promptDump) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dump dir: %w", err)
	}

	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dump: %w", err)
	}

	name := fmt.Sprintf("%s-%s-%02d.json",
		time.Now().UTC().Format("20060102T150405.000"), agentName, n)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write dump %s: %w", path, err)
	}

	log.Printf("coaching: prompt dump → %s (%d contents)", path, dump.ContentCount)
	return nil
}

// contentText flattens a Content into plain text, dropping non-text parts.
func contentText(c *genai.Content) string {
	if c == nil {
		return ""
	}

	var b strings.Builder
	for _, p := range c.Parts {
		if p != nil && p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// declaredToolNames lists the function declarations sent with the request;
// their schemas are part of the prompt's token cost too.
func declaredToolNames(tools []*genai.Tool) []string {
	var names []string
	for _, t := range tools {
		if t == nil {
			continue
		}
		for _, fn := range t.FunctionDeclarations {
			if fn != nil {
				names = append(names, fn.Name)
			}
		}
	}
	return names
}
