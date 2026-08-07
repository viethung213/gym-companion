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

// promptDumpDirEnv names a directory that receives two JSON files per model
// call — the request and the response. Unset disables both.
//
// ADK logs neither side, so without this an agent whose history filter admits
// more than intended looks identical to one that behaves. The file count also
// doubles as a per-run tally of model calls, which is what bounds the free
// tier's daily quota.
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

// responseDump is one captured model response.
type responseDump struct {
	Agent      string `json:"agent"`
	CapturedAt string `json:"captured_at"`

	// Error is set when the model call itself failed; the fields below are then
	// empty. Keeping the failure in the same stream as the successes is what
	// makes a retry sequence readable end to end.
	Error string `json:"error,omitempty"`

	FinishReason string `json:"finish_reason,omitempty"`

	// Text is what the agent actually answered, and FunctionCalls what it asked
	// to run instead. Exactly one of them is normally populated.
	Text          string        `json:"text,omitempty"`
	Thinking      string        `json:"thinking,omitempty"`
	FunctionCalls []toolRequest `json:"function_calls,omitempty"`

	Usage *usageDump `json:"usage,omitempty"`
}

// toolRequest is one tool the model asked to run.
type toolRequest struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// usageDump is the token accounting for one call. Thinking is billed as output
// but is not part of the answer, so it is reported separately: a reviewer that
// spends 1200 output tokens to return a 10-token verdict is only visible here.
type usageDump struct {
	PromptTokens   int32 `json:"prompt_tokens"`
	OutputTokens   int32 `json:"output_tokens"`
	ThinkingTokens int32 `json:"thinking_tokens"`
	CachedTokens   int32 `json:"cached_tokens"`
	TotalTokens    int32 `json:"total_tokens"`
}

// modelDumper writes the request and response of each model call as a matching
// pair of files. Request and response keep separate counters: the callbacks
// alternate one-to-one per call, so the numbers line up without either side
// having to read the other's state.
type modelDumper struct {
	dir       string
	agentName string

	reqSeq atomic.Int64
	resSeq atomic.Int64
}

// beforeModelCallbacks appends the opt-in request dumper to an agent's safety
// callbacks, leaving them untouched when dumping is disabled.
//
// agentName is passed in rather than read from the context: a callback
// context does not implement Agent().
func beforeModelCallbacks(agentName string, safety ...llmagent.BeforeModelCallback) []llmagent.BeforeModelCallback {
	d := newModelDumper(agentName)
	if d == nil {
		return safety
	}
	return append(safety, d.before)
}

// afterModelCallbacks returns the opt-in response dumper, empty when disabled.
func afterModelCallbacks(agentName string) []llmagent.AfterModelCallback {
	d := newModelDumper(agentName)
	if d == nil {
		return nil
	}
	return []llmagent.AfterModelCallback{d.after}
}

// newModelDumper returns nil when dumping is switched off, so a disabled dumper
// costs nothing at all rather than being a callback that returns early.
func newModelDumper(agentName string) *modelDumper {
	dir := strings.TrimSpace(os.Getenv(promptDumpDirEnv))
	if dir == "" {
		return nil
	}
	return &modelDumper{dir: dir, agentName: agentName}
}

// before captures the request. It always reports (nil, nil): a debugging aid
// must never suppress a model call, so every failure is logged and swallowed.
func (d *modelDumper) before(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req == nil {
		return nil, nil
	}

	dump := &promptDump{
		Agent:        d.agentName,
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

	if err := d.write("req", d.reqSeq.Add(1), dump); err != nil {
		log.Printf("coaching: request dump skipped: %v", err)
	}

	return nil, nil
}

// after captures the response, including a failed call. It always reports
// (nil, nil) so the real response or error passes through untouched.
func (d *modelDumper) after(_ agent.Context, res *model.LLMResponse, resErr error) (*model.LLMResponse, error) {
	dump := &responseDump{
		Agent:      d.agentName,
		CapturedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if resErr != nil {
		dump.Error = resErr.Error()
	}

	if res != nil {
		dump.FinishReason = string(res.FinishReason)
		dump.Text, dump.Thinking, dump.FunctionCalls = splitResponseParts(res.Content)
		dump.Usage = usageOf(res.UsageMetadata)
	}

	if err := d.write("res", d.resSeq.Add(1), dump); err != nil {
		log.Printf("coaching: response dump skipped: %v", err)
	}

	return nil, nil
}

// write names files so a lexical sort replays the call order, with each
// request adjacent to its response.
func (d *modelDumper) write(kind string, n int64, payload any) error {
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return fmt.Errorf("create dump dir: %w", err)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dump: %w", err)
	}

	name := fmt.Sprintf("%s-%s-%02d-%s.json",
		time.Now().UTC().Format("20060102T150405.000"), d.agentName, n, kind)
	path := filepath.Join(d.dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write dump %s: %w", path, err)
	}

	log.Printf("coaching: dump → %s", path)
	return nil
}

// splitResponseParts separates the answer from the thinking and the tool
// requests. Thought parts carry text too, so folding them together would make
// a reasoning trace look like the model's reply.
func splitResponseParts(c *genai.Content) (text, thinking string, calls []toolRequest) {
	if c == nil {
		return "", "", nil
	}

	var answer, thought strings.Builder
	for _, p := range c.Parts {
		switch {
		case p == nil:
		case p.FunctionCall != nil:
			calls = append(calls, toolRequest{Name: p.FunctionCall.Name, Args: p.FunctionCall.Args})
		case p.Text != "" && p.Thought:
			thought.WriteString(p.Text)
		case p.Text != "":
			answer.WriteString(p.Text)
		}
	}

	return answer.String(), thought.String(), calls
}

// usageOf copies the token counts, or nil when the model reported none.
func usageOf(m *genai.GenerateContentResponseUsageMetadata) *usageDump {
	if m == nil {
		return nil
	}

	return &usageDump{
		PromptTokens:   m.PromptTokenCount,
		OutputTokens:   m.CandidatesTokenCount + m.ThoughtsTokenCount,
		ThinkingTokens: m.ThoughtsTokenCount,
		CachedTokens:   m.CachedContentTokenCount,
		TotalTokens:    m.TotalTokenCount,
	}
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
