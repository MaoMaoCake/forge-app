package plugin

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	redis "github.com/redis/go-redis/v9"
)

//go:embed template/*.tpl
var templateFS embed.FS

// Make sure App implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. Plugin should not implement all these interfaces - only those which are
// required for a particular task.
var (
	_ backend.CallResourceHandler   = (*App)(nil)
	_ instancemgmt.InstanceDisposer = (*App)(nil)
	_ backend.CheckHealthHandler    = (*App)(nil)
)

// App is an example app plugin with a backend which can respond to data queries.
type App struct {
	backend.CallResourceHandler
	DB   *gorm.DB
	rdb  *redis.Client
	tmpl *template.Template
}

// NullString wraps sql.NullString to support JSON (de)serialization from/to
// plain JSON strings. It treats null or empty string as invalid.
type NullString struct {
	sql.NullString
}

func (ns *NullString) UnmarshalJSON(data []byte) error {
	// Handle JSON null
	if string(data) == "null" {
		ns.Valid = false
		ns.String = ""
		return nil
	}

	// Expect a JSON string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// If it's not a string, mark invalid but don't hard-fail
		ns.Valid = false
		ns.String = ""
		return nil
	}

	ns.String = s
	ns.Valid = s != ""
	return nil
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return json.Marshal("")
	}
	return json.Marshal(ns.String)
}

type Agent struct {
	gorm.Model
	Name        string      `gorm:"type:varchar(100);not null" json:"name"`
	AgentUUID   string      `gorm:"uniqueIndex" json:"agent_uuid"`
	LastSeen    time.Time   `json:"last_seen,omitempty"`
	Advanced    bool        `gorm:"default:false" json:"advanced"`
	AgentConfig AgentConfig `json:"agent_config"`
}

type AgentConfig struct {
	gorm.Model
	AgentID                  uint       `json:"-"`
	Config                   NullString `json:"config"`
	CollectUnixLogs          bool       `gorm:"default:false" json:"collectUnixLogs"`
	CollectUnixNodeMetrics   bool       `gorm:"default:false" json:"collectUnixNodeMetrics"`
	CollectWinLogs           bool       `gorm:"default:false" json:"collectWinLogs"`
	CollectWinNodeMetrics    bool       `gorm:"default:false" json:"collectWinNodeMetrics"`
	CollectCadvisorMetrics   bool       `gorm:"default:false" json:"collectCadvisorMetrics"`
	CollectKubernetesMetrics bool       `gorm:"default:false" json:"collectKubernetesMetrics"`
}

// NewApp creates a new example *App instance.
func NewApp(_ context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	var app App

	// Use a httpadapter (provided by the SDK) for resource calls. This allows us
	// to use a *http.ServeMux for resource calls, so we can map multiple routes
	// to CallResource without having to implement extra logic.
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	app.CallResourceHandler = httpadapter.New(mux)

	fmt.Println("postgresDsn:", settings.DecryptedSecureJSONData["postgresDsn"])
	// Read Postgres DSN from plugin JSON settings
	dsn, ok := settings.DecryptedSecureJSONData["postgresDsn"]
	if !ok || dsn == "" {
		return nil, fmt.Errorf("postgres DSN not configured; please set it in the plugin configuration page")
	}
	//redis_url, ok := settings.DecryptedSecureJSONData["redis_url"]
	//if !ok || redis_url == "" {
	//	return nil, fmt.Errorf("redis URL not configured; please set it in the plugin configuration page")
	//}

	fmt.Println("Initializing Postgres database with DSN", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		// Log the underlying error so you can see the actual cause in Grafana logs
		fmt.Println("DATABASE connection failed:", err)
		// Return the error instead of panicking so the plugin fails gracefully
		return nil, fmt.Errorf("open postgres db: %w", err)
	}

	if err := db.AutoMigrate(&Agent{}, &AgentConfig{}); err != nil {
		fmt.Println("DATABASE migration failed:", err)
		return nil, fmt.Errorf("auto migrate postgres db: %w", err)
	}

	app.DB = db

	// Parse templates from folder; use embedded assets so path lookup works in all environments
	tmpl := template.Must(template.ParseFS(templateFS, "template/*.tpl"))

	// Set up Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
	app.rdb = rdb
	app.tmpl = tmpl

	return &app, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created.
func (a *App) Dispose() {
	// cleanup
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
