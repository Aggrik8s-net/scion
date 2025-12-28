package agent

import (
	"context"
	"fmt"

	"github.com/ptone/scion-agent/pkg/api"
	"github.com/ptone/scion-agent/pkg/runtime"
)

type Manager interface {
	// Provision prepares the agent directory and configuration without starting it
	Provision(ctx context.Context, opts api.StartOptions) (*api.ScionConfig, error)

	// Start launches a new agent with the given configuration
	Start(ctx context.Context, opts api.StartOptions) (*api.AgentInfo, error)

	// Stop terminates an agent
	Stop(ctx context.Context, agentID string) error

	// Delete terminates and removes an agent
	Delete(ctx context.Context, agentID string, deleteFiles bool, grovePath string) error

	// List returns active agents
	List(ctx context.Context, filter map[string]string) ([]api.AgentInfo, error)

	// Watch returns a channel of status updates for an agent
	Watch(ctx context.Context, agentID string) (<-chan api.StatusEvent, error)
}

type AgentManager struct {
	Runtime runtime.Runtime
}

func NewManager(rt runtime.Runtime) Manager {
	return &AgentManager{
		Runtime: rt,
	}
}

func (m *AgentManager) Stop(ctx context.Context, agentID string) error {
	return m.Runtime.Stop(ctx, agentID)
}

func (m *AgentManager) Delete(ctx context.Context, agentID string, deleteFiles bool, grovePath string) error {
	if err := m.Runtime.Delete(ctx, agentID); err != nil {
		return err
	}
	if deleteFiles {
		return DeleteAgentFiles(agentID, grovePath)
	}
	return nil
}

func (m *AgentManager) Watch(ctx context.Context, agentID string) (<-chan api.StatusEvent, error) {
	return nil, fmt.Errorf("Watch not implemented")
}
