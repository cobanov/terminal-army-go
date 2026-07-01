package svc

import "time"

// User mirrors the public-facing user record returned by /api/v1/me.
type User struct {
	ID                int64      `json:"id"`
	Username          string     `json:"username"`
	Email             string     `json:"email"`
	Role              string     `json:"role"`
	CurrentUniverseID *int64     `json:"current_universe_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	LastSeenAt        *time.Time `json:"last_seen_at,omitempty"`
}

// Session represents a device session row resolved from a JWT.
type Session struct {
	ID         int64
	UserID     int64
	User       *User
	DeviceName string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// AuthResult is returned by Register and Login.
type AuthResult struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// DeviceAuthStart is returned to the CLI before browser auth begins.
type DeviceAuthStart struct {
	AuthCode        string `json:"auth_code"`
	ExpiresIn       int    `json:"expires_in"`
	PollingInterval int    `json:"polling_interval"`
}

// DeviceAuthPoll is returned when browser auth has completed.
type DeviceAuthPoll struct {
	Token string `json:"token"`
}

// Universe is the public view of a game universe.
type Universe struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	SpeedEconomy  int       `json:"speed_economy"`
	SpeedFleet    int       `json:"speed_fleet"`
	SpeedResearch int       `json:"speed_research"`
	GalaxiesCount int       `json:"galaxies_count"`
	SystemsCount  int       `json:"systems_count"`
	PlayerCount   int       `json:"player_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Planet is the public view of a planet owned by the current user.
type Planet struct {
	ID                     int64          `json:"id"`
	Code                   string         `json:"code"`
	Name                   string         `json:"name"`
	OwnerUserID            int64          `json:"owner_user_id"`
	UniverseID             int64          `json:"universe_id"`
	Galaxy                 int            `json:"galaxy"`
	System                 int            `json:"system"`
	Position               int            `json:"position"`
	FieldsUsed             int            `json:"fields_used"`
	FieldsTotal            int            `json:"fields_total"`
	TempMin                int            `json:"temp_min"`
	TempMax                int            `json:"temp_max"`
	Metal                  float64        `json:"metal"`
	Crystal                float64        `json:"crystal"`
	Deuterium              float64        `json:"deuterium"`
	EnergyUsed             int            `json:"energy_used"`
	EnergyProduced         int            `json:"energy_produced"`
	ResourcesLastUpdatedAt time.Time      `json:"resources_last_updated_at"`
	Buildings              map[string]int `json:"buildings"`
	Ships                  map[string]int `json:"ships,omitempty"`
	Defense                map[string]int `json:"defense,omitempty"`
}

// ProductionReport summarises a planet's per-resource production rates.
type ProductionReport struct {
	PlanetID            int64   `json:"planet_id"`
	MetalPerHour        float64 `json:"metal_per_hour"`
	CrystalPerHour      float64 `json:"crystal_per_hour"`
	DeuteriumPerHour    float64 `json:"deuterium_per_hour"`
	EnergyProduced      int     `json:"energy_produced"`
	EnergyUsed          int     `json:"energy_used"`
	ProductionFactor    float64 `json:"production_factor"`
	StorageCapMetal     int     `json:"storage_cap_metal"`
	StorageCapCrystal   int     `json:"storage_cap_crystal"`
	StorageCapDeuterium int     `json:"storage_cap_deuterium"`
}

// Cost is a resolved resource cost for one build/research/unit action. Energy
// is the marginal energy the action consumes (mines) and is 0 for everything
// else.
type Cost struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
	Energy    int     `json:"energy,omitempty"`
}

// BuildingView is a render-ready row for the buildings / facilities screens.
// Every field the client needs to draw the row and decide whether the action
// is available is resolved server-side, so the TUI never imports internal/game.
type BuildingView struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Category     string `json:"category"` // resource | facility
	Level        int    `json:"level"`
	NextCost     Cost   `json:"next_cost"`
	BuildSeconds int    `json:"build_seconds"`
	Affordable   bool   `json:"affordable"`
	Locked       bool   `json:"locked"`
	LockedReason string `json:"locked_reason,omitempty"`
}

// UnitView is a render-ready row for the shipyard / defense screens.
type UnitView struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Owned        int    `json:"owned"`
	UnitCost     Cost   `json:"unit_cost"`
	BuildSeconds int    `json:"build_seconds"` // per-unit build time
	BuildableNow int    `json:"buildable_now"` // how many current resources allow
	Locked       bool   `json:"locked"`
	LockedReason string `json:"locked_reason,omitempty"`
}

// ResearchNode is one row in the research view, with tree parent resolved.
type ResearchNode struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Level        int    `json:"level"`
	NextCost     Cost   `json:"next_cost"`
	BuildSeconds int    `json:"build_seconds"`
	Parent       string `json:"parent,omitempty"`
	Affordable   bool   `json:"affordable"`
	Locked       bool   `json:"locked"`
	LockedReason string `json:"locked_reason,omitempty"`
}

// ResearchView is the response to GET /planets/{id}/research. LabLevel is the
// highest Research Lab across the user's planets (the research-time ceiling).
type ResearchView struct {
	LabLevel int            `json:"lab_level"`
	Nodes    []ResearchNode `json:"nodes"`
}

// QueueItem is one row in a planet's build / shipyard / research queue.
type QueueItem struct {
	ID          int64     `json:"id"`
	PlanetID    *int64    `json:"planet_id,omitempty"`
	UserID      int64     `json:"user_id"`
	QueueType   string    `json:"queue_type"` // building | research | ship | defense
	ItemKey     string    `json:"item_key"`
	TargetLevel int       `json:"target_level,omitempty"`
	Count       int       `json:"count,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
}

// ResearchLevel is one technology level for a user.
type ResearchLevel struct {
	Tech  string `json:"tech"`
	Level int    `json:"level"`
}

// FleetDispatchRequest is the body of POST /api/v1/fleet.
type FleetDispatchRequest struct {
	OriginPlanetID int64          `json:"origin_planet_id"`
	TargetGalaxy   int            `json:"target_galaxy"`
	TargetSystem   int            `json:"target_system"`
	TargetPosition int            `json:"target_position"`
	Mission        string         `json:"mission"`
	Ships          map[string]int `json:"ships"`
	Cargo          map[string]int `json:"cargo,omitempty"`
	SpeedPercent   int            `json:"speed_percent,omitempty"`
}

// Fleet is a public view of an in-flight or stationed fleet.
type Fleet struct {
	ID             int64          `json:"id"`
	UserID         int64          `json:"user_id"`
	OriginPlanetID int64          `json:"origin_planet_id"`
	TargetGalaxy   int            `json:"target_galaxy"`
	TargetSystem   int            `json:"target_system"`
	TargetPosition int            `json:"target_position"`
	Mission        string         `json:"mission"`
	State          string         `json:"state"`
	DepartureAt    time.Time      `json:"departure_at"`
	ArrivalAt      time.Time      `json:"arrival_at"`
	ReturnAt       *time.Time     `json:"return_at,omitempty"`
	Ships          map[string]int `json:"ships"`
	Cargo          map[string]int `json:"cargo,omitempty"`
}

// Message is a player notification or in-game communication.
type Message struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Category  string    `json:"category"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// Report holds a combat or espionage report payload.
type Report struct {
	ID        int64          `json:"id"`
	UserID    int64          `json:"user_id"`
	Kind      string         `json:"kind"`
	Subject   string         `json:"subject"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

// Alliance is the public view of an alliance.
type Alliance struct {
	ID          int64     `json:"id"`
	Tag         string    `json:"tag"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerUserID int64     `json:"owner_user_id"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// LeaderboardEntry is one row in the global leaderboard.
type LeaderboardEntry struct {
	Rank     int    `json:"rank"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Score    int64  `json:"score"`
	Alliance string `json:"alliance,omitempty"`
}

// StatsOverview is the response to /api/v1/stats.
type StatsOverview struct {
	Universes      int   `json:"universes"`
	Players        int   `json:"players"`
	Planets        int   `json:"planets"`
	OnlinePlayers  int   `json:"online_players"`
	FleetsInFlight int   `json:"fleets_in_flight"`
	UptimeSeconds  int64 `json:"uptime_seconds"`
}

// PublicServerStats mirrors the Python lobby /stats shape used by existing
// installers and terminal clients.
type PublicServerStats struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxUsers    int    `json:"max_users"`
	Registered  int    `json:"registered"`
	Online      int    `json:"online"`
	Active24h   int    `json:"active_24h"`
	Full        bool   `json:"full"`
	Version     string `json:"version"`
}

// SystemPlanetView is one slot in a galaxy system view.
type SystemPlanetView struct {
	Position    int    `json:"position"`
	PlanetName  string `json:"planet_name,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
	AllianceTag string `json:"alliance_tag,omitempty"`
	Online      bool   `json:"online,omitempty"`
	Score       int64  `json:"score,omitempty"`
}

// SystemView is the response to /api/v1/galaxy/{g}/{s}.
type SystemView struct {
	Galaxy  int                `json:"galaxy"`
	System  int                `json:"system"`
	Planets []SystemPlanetView `json:"planets"`
}
