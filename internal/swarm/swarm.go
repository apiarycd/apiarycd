package swarm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/moby/moby/client"
	"go.uber.org/zap"
)

// Swarm wraps Swarm-specific operations for the Docker client.
type Swarm struct {
	client *client.Client
	logger *zap.Logger
}

// NewSwarm creates a new Swarm wrapper.
func NewSwarm(client *client.Client, logger *zap.Logger) *Swarm {
	return &Swarm{
		client: client,
		logger: logger,
	}
}

// DeployStack deploys a Docker stack using a compose file.
func (s *Swarm) DeployStack(ctx context.Context, req DeployStackRequest) ([]string, error) {
	s.logger.Info(
		"Deploying Docker stack",
		zap.String("name", req.StackName),
		zap.String("compose_path", req.ComposePath),
	)

	//nolint:gosec // skip for now
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"stack",
		"deploy",
		"--compose-file",
		req.ComposePath,
		"--with-registry-auth",
		req.StackName,
	)
	cmd.Dir = req.WorkDir
	cmd.Env = prepareEnv(req.Env)

	output, err := cmd.CombinedOutput()
	logs := splitLines(output)
	if err != nil {
		s.logger.Error(
			"Failed to deploy Docker stack",
			zap.String("name", req.StackName),
			zap.String("output", strings.TrimSpace(string(output))),
			zap.Error(err),
		)
		return logs, fmt.Errorf("failed to deploy stack %q: %w", req.StackName, err)
	}

	s.logger.Info("Docker stack deployed successfully", zap.String("name", req.StackName))
	return logs, nil
}

// RemoveStack removes a deployed Docker stack by name.
func (s *Swarm) RemoveStack(ctx context.Context, stackName string) error {
	s.logger.Info("Removing Docker stack", zap.String("name", stackName))

	cmd := exec.CommandContext(ctx, "docker", "stack", "rm", stackName)
	cmd.Env = prepareEnv(nil)

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error(
			"Failed to remove Docker stack",
			zap.String("name", stackName),
			zap.String("output", strings.TrimSpace(string(output))),
			zap.Error(err),
		)
		return fmt.Errorf("failed to remove stack %q: %w", stackName, err)
	}

	s.logger.Info("Docker stack removed successfully", zap.String("name", stackName))
	return nil
}

func prepareEnv(src []string) []string {
	// Use minimal base environment to avoid leaking sensitive host variables
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	// Include DOCKER_* variables for Docker client configuration
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "DOCKER_") {
			env = append(env, e)
		}
	}

	if len(src) > 0 {
		for _, e := range src {
			key, _, _ := strings.Cut(e, "=")
			if key == "PATH" || key == "HOME" || strings.HasPrefix(key, "DOCKER_") {
				continue
			}
			env = append(env, e)
		}
	}

	return env
}

func splitLines(output []byte) []string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	return lines
}
