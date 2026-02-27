package plugin

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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
	DB *gorm.DB
}

type Agent struct {
	gorm.Model
	Name        string `gorm:"type:varchar(100);not null"`
	AgentUUID   string `gorm:"uniqueIndex"`
	LastSeen    time.Time
	Advanced    bool `gorm:"default:false"`
	AgentConfig AgentConfig
}

type AgentConfig struct {
	gorm.Model
	AgentID                  uint
	Config                   sql.NullString // if Advanced is true read this. Otherwise, Render according to flags
	CollectUnixLogs          bool           `gorm:"default:false"`
	CollectUnixNodeMetrics   bool           `gorm:"default:false"`
	CollectWinLogs           bool           `gorm:"default:false"`
	CollectWinNodeMetrics    bool           `gorm:"default:false"`
	CollectCadvisorMetrics   bool           `gorm:"default:false"`
	CollectKubernetesMetrics bool           `gorm:"default:false"`
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
