package adk

import (
	"context"
	"os"

	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/skilltoolset"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

func makeInjuryRecoverySkillToolset(ctx context.Context) (tool.Toolset, error) {
	source := skill.NewFileSystemSource(os.DirFS("internal/coaching/infrastructure/ai/adk/skills"))
	return skilltoolset.New(ctx, skilltoolset.Config{
		Source: source,
	})
}
