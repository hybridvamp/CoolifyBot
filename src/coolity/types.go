package coolify

type Application struct {
	ID     int64  `json:"id"`
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	FQDN   string `json:"fqdn"`
	Status string `json:"status"`
}

type ApplicationDetail struct {
	ID                      int64  `json:"id"`
	UUID                    string `json:"uuid"`
	Name                    string `json:"name"`
	FQDN                    string `json:"fqdn"`
	Status                  string `json:"status"`
	Description             string `json:"description"`
	GitRepository           string `json:"git_repository"`
	GitBranch               string `json:"git_branch"`
	DockerRegistryImageName string `json:"docker_registry_image_name"`
	Dockerfile              string `json:"dockerfile"`
	BuildPack               string `json:"build_pack"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
	// Additional resource associations
	Environment string `json:"environment"`
}

type ApplicationLogs struct {
	Logs string `json:"logs"`
}

type EnvironmentVariable struct {
	ID               int64  `json:"id"`
	UUID             string `json:"uuid"`
	ResourceableType string `json:"resourceable_type"`
	ResourceableID   int64  `json:"resourceable_id"`
	IsBuildTime      bool   `json:"is_build_time"`
	IsLiteral        bool   `json:"is_literal"`
	IsMultiline      bool   `json:"is_multiline"`
	IsPreview        bool   `json:"is_preview"`
	IsShared         bool   `json:"is_shared"`
	IsShownOnce      bool   `json:"is_shown_once"`
	Key              string `json:"key"`
	Value            string `json:"value"`
	RealValue        string `json:"real_value"`
	Version          string `json:"version"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type StartDeploymentResponse struct {
	Message        string `json:"message"`
	DeploymentUUID string `json:"deployment_uuid"`
}

type StopApplicationResponse struct {
	Message string `json:"message"`
}

type Deployment struct {
	UUID          string `json:"uuid"`
	Status        string `json:"status"`
	Commit        string `json:"commit"`
	Branch        string `json:"branch"`
	CommitMessage string `json:"commit_message"`
	Type          string `json:"type"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	ApplicationID int64  `json:"application_id"`
	Application   string `json:"application"`
}

type Environment struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ProjectID   int64  `json:"project_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Database struct {
	ID        int64  `json:"id"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Pagination struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

type Meta struct {
	Pagination Pagination `json:"pagination"`
}

type Page[T any] struct {
	Data         []T        `json:"data"`
	Items        []T        `json:"items"`
	Applications []T        `json:"applications"` // compatibility with older Coolify responses
	Pagination   Pagination `json:"pagination"`
	Meta         Meta       `json:"meta"`
}

func (p Page[T]) Results() []T {
	switch {
	case len(p.Data) > 0:
		return p.Data
	case len(p.Items) > 0:
		return p.Items
	case len(p.Applications) > 0:
		return p.Applications
	default:
		return nil
	}
}

func (p Page[T]) PageInfo() Pagination {
	if p.Meta.Pagination != (Pagination{}) {
		return p.Meta.Pagination
	}
	return p.Pagination
}
