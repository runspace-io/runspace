package hosttransport

import "time"

type PresenceState string

const (
	PresenceOnline   PresenceState = "online"
	PresenceDegraded PresenceState = "degraded"
	PresenceOffline  PresenceState = "offline"
)

type AgentState string

const (
	AgentReady           AgentState = "ready"
	AgentBusy            AgentState = "busy"
	AgentWaitingApproval AgentState = "waiting_approval"
	AgentAdapterMissing  AgentState = "adapter_missing"
	AgentCrashed         AgentState = "crashed"
)

type HealthReport struct {
	DeviceID    string                 `json:"device_id"`
	OwnerUserID string                 `json:"owner_user_id"`
	Presence    PresenceState          `json:"presence"`
	Route       Route                  `json:"route"`
	Agents      map[string]AgentHealth `json:"agents"`
	Resources   map[string]string      `json:"resources"`
	ObservedAt  time.Time              `json:"observed_at"`
	LeaseUntil  time.Time              `json:"lease_until"`
}

type AgentHealth struct {
	State           AgentState `json:"state"`
	SessionCount    int        `json:"session_count"`
	ContextPressure int        `json:"context_pressure_percent,omitempty"`
	LastErrorCode   string     `json:"last_error_code,omitempty"`
}

func (report HealthReport) EffectivePresence(now time.Time) PresenceState {
	if report.LeaseUntil.IsZero() || !now.Before(report.LeaseUntil) {
		return PresenceOffline
	}
	return report.Presence
}
