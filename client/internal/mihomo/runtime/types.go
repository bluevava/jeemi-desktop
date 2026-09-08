package runtime

import (
	"jeemi/internal/config/dnstool"
	"jeemi/internal/runtimeconfig"
)

type State string

const (
	StateStopped    State = "stopped"
	StatePreparing  State = "preparing"
	StateValidating State = "validating"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateReloading  State = "reloading"
	StateStopping   State = "stopping"
	StateRecovering State = "recovering"
	StateFailed     State = "failed"
)

type Failure struct {
	Code    string `json:"code"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type ControllerSession struct {
	ID      string `json:"id"`
	BaseURL string `json:"baseUrl"`
	Secret  string `json:"secret"`
}

type Status struct {
	State             State              `json:"state"`
	DesiredRunning    bool               `json:"desiredRunning"`
	CoreVersion       string             `json:"coreVersion"`
	PID               int                `json:"pid"`
	GenerationID      string             `json:"generationId"`
	SubscriptionID    string             `json:"subscriptionId"`
	Source            Source             `json:"source"`
	ControllerReady   bool               `json:"controllerReady"`
	ControllerSession *ControllerSession `json:"controllerSession"`
	OutboundMode      string             `json:"outboundMode"`
	ProxyMode         string             `json:"proxyMode"`
	SystemProxy       bool               `json:"systemProxy"`
	TUNEnabled        bool               `json:"tunEnabled"`
	TUNDevice         string             `json:"tunDevice"`
	StartedAt         string             `json:"startedAt"`
	RestartAttempt    int                `json:"restartAttempt"`
	LastError         *Failure           `json:"lastError"`
}

type Source struct {
	NormalizationFingerprint     string `json:"normalizationFingerprint"`
	SubscriptionID               string `json:"subscriptionId"`
	SubscriptionRevision         string `json:"subscriptionRevision"`
	LocalConfigID                string `json:"localConfigId"`
	LocalConfigRevision          int    `json:"localConfigRevision"`
	LocalScriptID                string `json:"localScriptId"`
	LocalScriptRevision          int    `json:"localScriptRevision"`
	RuleProviderOverrideRevision int    `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int    `json:"fallbackOverrideRevision"`
	GeoDataFingerprint           string `json:"geoDataFingerprint"`
}

type StartRequest struct {
	ExecutablePath string
	CoreVersion    string
	Configuration  []byte
	Preferences    runtimeconfig.Preferences
	Source         Source
}

type Generation struct {
	DNS            dnstool.Session
	ID             string
	Directory      string
	ConfigPath     string
	BootstrapPath  string
	CoreVersion    string
	ExecutablePath string
	Preferences    runtimeconfig.Preferences
	Source         Source
	Session        ControllerSession
}
