package config

// Stage represents the deployment environment.
type Stage string

const (
	StageLocal Stage = "local"
	StageDev   Stage = "dev"
	StageProd  Stage = "prod"
)

// stageFromEnv converts an env string to a Stage.
func stageFromEnv(env string) Stage {
	switch env {
	case "prod", "production":
		return StageProd
	case "dev", "development":
		return StageDev
	default:
		return StageLocal
	}
}

// String returns the string representation.
func (s Stage) String() string {
	return string(s)
}
