package swarm

type DeployStackRequest struct {
	StackName   string
	ComposePath string
	WorkDir     string
	Env         []string
}
