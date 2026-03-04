package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	connect "connectrpc.com/connect"
	v1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	collectorv1connect "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
)

const (
	configTemplateName = "config.alloy.tpl"
	defaultTenantID    = "default"
)

type serviceEndpoint struct {
	URL string `json:"url"`
}

type LGTMCFG struct {
	Mimir serviceEndpoint `json:"mimir"`
	Loki  serviceEndpoint `json:"loki"`
	Tempo serviceEndpoint `json:"tempo"`
}

type Features struct {
	SelfMonitor      bool
	LinuxMonitor     bool
	WindowsMonitor   bool
	ContainerMonitor bool
	JournalLog       bool
	WindowsEventLog  bool
	DockerLog        bool
	FileMonitor      bool
}

// handlePing is an example HTTP GET resource that returns a {"message": "ok"} JSON response.
func (a *App) handlePing(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"message": "ok"}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleEcho is an example HTTP POST resource that accepts a JSON with a "message" key and
// returns to the client whatever it is sent.
func (a *App) handleEcho(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) getCollector(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received request for /collector")
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ensure DB is initialized
	if a.DB == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	uuid := req.URL.Query().Get("uuid")

	w.Header().Add("Content-Type", "application/json")

	if uuid != "" {
		var agent Agent
		if err := a.DB.Preload("AgentConfig").Where("agent_uuid = ?", uuid).First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				http.Error(w, "collector not found", http.StatusNotFound)
				return
			}
			fmt.Println("error querying agent:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(agent); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// For now, return all agents with their configs. You can later filter
	// by query parameters (e.g., agent UUID) if needed.
	var agents []Agent
	if err := a.DB.Preload("AgentConfig").Find(&agents).Error; err != nil {
		fmt.Println("error querying agents:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(agents); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) createOrUpdateAgent(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received request for /collector (create/update)")

	var agent Agent
	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if a.DB == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var existing Agent
		if err := tx.Where("agent_uuid = ?", agent.AgentUUID).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new agent
				return tx.Create(&agent).Error
			}
			return err
		}

		// Update existing agent
		existing.Name = agent.Name
		existing.LastSeen = agent.LastSeen
		existing.Advanced = agent.Advanced

		if err := tx.Save(&existing).Error; err != nil {
			return err
		}

		// Upsert AgentConfig
		var existingConfig AgentConfig
		if err := tx.Where("agent_id = ?", existing.ID).First(&existingConfig).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				agent.AgentConfig.AgentID = existing.ID
				return tx.Create(&agent.AgentConfig).Error
			}
			return err
		}

		existingConfig.Config = agent.AgentConfig.Config
		existingConfig.CollectUnixLogs = agent.AgentConfig.CollectUnixLogs
		existingConfig.CollectUnixNodeMetrics = agent.AgentConfig.CollectUnixNodeMetrics
		existingConfig.CollectWinLogs = agent.AgentConfig.CollectWinLogs
		existingConfig.CollectWinNodeMetrics = agent.AgentConfig.CollectWinNodeMetrics
		existingConfig.CollectCadvisorMetrics = agent.AgentConfig.CollectCadvisorMetrics
		existingConfig.CollectKubernetesMetrics = agent.AgentConfig.CollectKubernetesMetrics

		return tx.Save(&existingConfig).Error
	})

	if err != nil {
		fmt.Println("error upserting agent:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *App) GetConfig(ctx context.Context, req *connect.Request[v1.GetConfigRequest]) (*connect.Response[v1.GetConfigResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("request payload is required"))
	}
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("collector id is required"))
	}

	tenantID := tenantIDFromAttrs(req.Msg.LocalAttributes)
	if tenantID == "" {
		tenantID = os.Getenv("DEFAULT_TENANT_ID")
	}
	if tenantID == "" {
		tenantID = defaultTenantID
	}

	body, err := a.render(ctx, req, tenantID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hash := hashConfig(body)
	if req.Msg.Hash != "" && req.Msg.Hash == hash {
		return connect.NewResponse(&v1.GetConfigResponse{
			Hash:        hash,
			NotModified: true,
		}), nil
	}

	return connect.NewResponse(&v1.GetConfigResponse{
		Content: body,
		Hash:    hash,
	}), nil
}

func (a *App) RegisterCollector(ctx context.Context, req *connect.Request[v1.RegisterCollectorRequest]) (*connect.Response[v1.RegisterCollectorResponse], error) {
	log.Printf("RegisterCollector: id=%s local_attrs=%v", req.Msg.Id, req.Msg.LocalAttributes)
	return connect.NewResponse(&v1.RegisterCollectorResponse{}), nil
}

func (s *App) render(ctx context.Context, req *connect.Request[v1.GetConfigRequest], tenantID string) (string, error) {
	if s.rdb == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	if s.tmpl == nil {
		return "", fmt.Errorf("template bundle not initialized")
	}

	featureKey := fmt.Sprintf("nova:feat:%s:%s", tenantID, req.Msg.Id)
	featureString, err := s.rdb.Get(ctx, featureKey).Result()
	var features Features
	if err != nil {
		if err != redis.Nil {
			return "", fmt.Errorf("fetch features: %w", err)
		}
	} else {
		features, err = parseFeatures(featureString)
		if err != nil {
			return "", fmt.Errorf("parse features: %w", err)
		}
	}

	credentials := map[string]string{
		"mimir_password": os.Getenv("MIMIR_PASSWORD"),
		"loki_password":  os.Getenv("LOKI_PASSWORD"),
		"tempo_password": os.Getenv("TEMPO_PASSWORD"),
	}
	for key, value := range credentials {
		if value == "" {
			credentials[key] = "password"
		}
	}

	lgtmcfg := LGTMCFG{
		Mimir: serviceEndpoint{URL: os.Getenv("MIMIR_URL")},
		Loki:  serviceEndpoint{URL: os.Getenv("LOKI_URL")},
		Tempo: serviceEndpoint{URL: os.Getenv("TEMPO_URL")},
	}

	data := map[string]any{
		"TenantID":       tenantID,
		"ID":             req.Msg.Id,
		"LocalAttrs":     req.Msg.LocalAttributes,
		"RequestedHash":  req.Msg.Hash,
		"RequestedAtUTC": time.Now().UTC().Format(time.RFC3339),
		"Features":       features,
		"Credentials":    credentials,
		"LGTMCfg":        lgtmcfg,
	}
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, configTemplateName, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

func (a *App) UnregisterCollector(ctx context.Context, req *connect.Request[v1.UnregisterCollectorRequest]) (*connect.Response[v1.UnregisterCollectorResponse], error) {
	log.Printf("UnregisterCollector: id=%s", req.Msg.Id)
	return connect.NewResponse(&v1.UnregisterCollectorResponse{}), nil
}

// registerRoutes takes a *http.ServeMux and registers some HTTP handlers.
func (a *App) registerRoutes(mux *http.ServeMux) {
	path, handler := collectorv1connect.NewCollectorServiceHandler(a)
	fmt.Println(path)
	mux.Handle(path, handler)

	mux.HandleFunc("/ping", a.handlePing)
	mux.HandleFunc("/echo", a.handleEcho)

	collectorHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			a.getCollector(w, r)
			return
		}

		if r.Method == http.MethodPost {
			a.createOrUpdateAgent(w, r)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

	mux.HandleFunc("/collector", collectorHandler)
}

func parseFeatures(raw string) (Features, error) {
	if strings.TrimSpace(raw) == "" {
		return Features{}, nil
	}

	var generic map[string]bool
	if err := json.Unmarshal([]byte(raw), &generic); err == nil {
		return featuresFromMap(generic), nil
	}

	var structured Features
	if err := json.Unmarshal([]byte(raw), &structured); err != nil {
		return Features{}, err
	}
	return structured, nil
}

func featuresFromMap(in map[string]bool) Features {
	var f Features
	for key, value := range in {
		switch strings.ToLower(key) {
		case "selfmonitor", "self_monitor":
			f.SelfMonitor = value
		case "linuxmonitor", "linux_monitor":
			f.LinuxMonitor = value
		case "windowsmonitor", "windows_monitor":
			f.WindowsMonitor = value
		case "containermonitor", "container_monitor":
			f.ContainerMonitor = value
		case "journallog", "journal_log":
			f.JournalLog = value
		case "windowseventlog", "windows_event_log":
			f.WindowsEventLog = value
		case "dockerlog", "docker_log":
			f.DockerLog = value
		case "filemonitor", "file_monitor":
			f.FileMonitor = value
		}
	}
	return f
}

func tenantIDFromAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	preferred := []string{"tenant_id", "tenantId", "tenant", "org_id", "orgId", "x-scope-orgid"}
	for _, key := range preferred {
		if val, ok := attrs[key]; ok && val != "" {
			return val
		}
	}
	for key, val := range attrs {
		if strings.EqualFold(key, "tenant") || strings.EqualFold(key, "tenant_id") || strings.EqualFold(key, "x-scope-orgid") {
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func hashConfig(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
