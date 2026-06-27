package tools

import (
	"context"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

type Tool interface {
	Name() string
	Definition() llm.ToolDefinition
	Execute(ctx context.Context, arguments string) (string, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Definitions(names []string) []llm.ToolDefinition {
	var defs []llm.ToolDefinition
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			defs = append(defs, t.Definition())
		}
	}
	return defs
}
