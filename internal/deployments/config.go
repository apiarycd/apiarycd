package deployments

import "time"

type Config struct {
	DeployTimeout            time.Duration
	RotateImmutableResources bool
}
