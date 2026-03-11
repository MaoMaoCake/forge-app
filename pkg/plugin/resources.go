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
	// Can be overridden with REDIS_RENDERED_CONFIG_KEY_TEMPLATE env var.
	defaultRenderedConfigKeyTemplate = "nova:cfg:%s:%s"
)

var errCollectorNotFound = errors.New("collector not found")
var collectorVersionKeyNormalizer = strings.NewReplacer(".", "", "_", "", "-", "", " ", "")

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
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"message": "ok"}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

	w.Header().Set("Content-Type", "application/json")

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
}

func (a *App) createOrUpdateAgent(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received request for /collector (create/update)")

	var agent Agent
	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	agent.AgentUUID = strings.TrimSpace(agent.AgentUUID)
	agent.LastSeenVersion = strings.TrimSpace(agent.LastSeenVersion)
	if agent.AgentUUID == "" {
		http.Error(w, "agent_uuid is required", http.StatusBadRequest)
		return
	}

	if a.DB == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var existing Agent
		if err := tx.Where("agent_uuid = ?", agent.AgentUUID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create a new agent
				if agent.LastSeen.IsZero() {
					agent.LastSeen = now
				}
				if strings.TrimSpace(agent.Name) == "" {
					agent.Name = agent.AgentUUID
				}
				return tx.Create(&agent).Error
			}
			return err
		}

		// Update existing agent
		existing.Name = strings.TrimSpace(agent.Name)
		if existing.Name == "" {
			existing.Name = existing.AgentUUID
		}
		if agent.LastSeen.IsZero() {
			existing.LastSeen = now
		} else {
			existing.LastSeen = agent.LastSeen
		}
		if agent.LastSeenVersion != "" {
			existing.LastSeenVersion = agent.LastSeenVersion
		}
		existing.Advanced = agent.Advanced

		if err := tx.Save(&existing).Error; err != nil {
			return err
		}

		// Upsert AgentConfig
		var existingConfig AgentConfig
		if err := tx.Where("agent_id = ?", existing.ID).First(&existingConfig).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
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

	tenantID := resolveTenantID("")
	if _, err := a.renderAndStoreCollectorConfig(req.Context(), tenantID, agent.AgentUUID, nil, "", true); err != nil {
		fmt.Println("error rendering/storing collector config:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *App) GetConfig(ctx context.Context, req *connect.Request[v1.GetConfigRequest]) (*connect.Response[v1.GetConfigResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("request payload is required"))
	}
	collectorID := strings.TrimSpace(req.Msg.Id)
	if collectorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("collector id is required"))
	}
	req.Msg.Id = collectorID

	tenantID := resolveTenantID(tenantIDFromAttrs(req.Msg.LocalAttributes))

	body, err := a.render(ctx, req, tenantID)
	if err != nil {
		if errors.Is(err, errCollectorNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("collector %q not found", collectorID))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := a.updateCollectorLastSeen(ctx, collectorID); err != nil {
		log.Printf("GetConfig: failed to update collector last seen: id=%s err=%v", collectorID, err)
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
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("request payload is required"))
	}
	collectorID := strings.TrimSpace(req.Msg.Id)
	if collectorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("collector id is required"))
	}
	if a.DB == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database not initialized"))
	}

	log.Printf("RegisterCollector: id=%s local_attrs=%v", collectorID, req.Msg.LocalAttributes)
	version := collectorVersionFromAttrs(req.Msg.LocalAttributes)
	if version == "" {
		version = collectorVersionFromAttrs(req.Msg.Attributes)
	}

	now := time.Now().UTC()
	err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Agent
		err := tx.Where("agent_uuid = ?", collectorID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			name := strings.TrimSpace(req.Msg.Name)
			if name == "" {
				name = collectorID
			}
			return tx.Create(&Agent{
				Name:            name,
				AgentUUID:       collectorID,
				LastSeen:        now,
				LastSeenVersion: version,
			}).Error
		}
		if err != nil {
			return err
		}

		existing.LastSeen = now
		if version != "" && version != existing.LastSeenVersion {
			existing.LastSeenVersion = version
		}
		if name := strings.TrimSpace(req.Msg.Name); name != "" {
			existing.Name = name
		} else if strings.TrimSpace(existing.Name) == "" {
			existing.Name = collectorID
		}
		return tx.Save(&existing).Error
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("register collector: %w", err))
	}

	return connect.NewResponse(&v1.RegisterCollectorResponse{}), nil
}

func (a *App) updateCollectorLastSeen(ctx context.Context, collectorID string) error {
	if a.DB == nil {
		return nil
	}

	updates := map[string]any{
		"last_seen": time.Now().UTC(),
	}

	result := a.DB.WithContext(ctx).Model(&Agent{}).Where("agent_uuid = ?", collectorID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update collector heartbeat: %w", result.Error)
	}

	return nil
}

func (a *App) render(ctx context.Context, req *connect.Request[v1.GetConfigRequest], tenantID string) (string, error) {
	return a.renderAndStoreCollectorConfig(
		ctx,
		tenantID,
		req.Msg.Id,
		req.Msg.LocalAttributes,
		req.Msg.Hash,
		false,
	)
}

func (a *App) UnregisterCollector(ctx context.Context, req *connect.Request[v1.UnregisterCollectorRequest]) (*connect.Response[v1.UnregisterCollectorResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("request payload is required"))
	}
	collectorID := strings.TrimSpace(req.Msg.Id)
	if collectorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("collector id is required"))
	}
	log.Printf("UnregisterCollector: id=%s", collectorID)
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

func featuresFromAgentConfig(cfg AgentConfig) Features {
	return Features{
		LinuxMonitor:     cfg.CollectUnixNodeMetrics,
		WindowsMonitor:   cfg.CollectWinNodeMetrics,
		ContainerMonitor: cfg.CollectCadvisorMetrics,
		JournalLog:       cfg.CollectUnixLogs,
		WindowsEventLog:  cfg.CollectWinLogs,
	}
}

func (a *App) featuresForCollector(ctx context.Context, tenantID, collectorID string) (Features, string, error) {
	if a.DB != nil {
		var agent Agent
		err := a.DB.WithContext(ctx).Preload("AgentConfig").Where("agent_uuid = ?", collectorID).First(&agent).Error
		if err == nil {
			if agent.Advanced {
				customConfig := strings.TrimSpace(agent.AgentConfig.Config.String)
				if customConfig != "" {
					return Features{}, customConfig, nil
				}
			}
			return featuresFromAgentConfig(agent.AgentConfig), "", nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Features{}, "", fmt.Errorf("fetch collector from database: %w", err)
		}
	}

	if a.rdb == nil {
		return Features{}, "", errCollectorNotFound
	}

	featureKey := fmt.Sprintf("nova:feat:%s:%s", tenantID, collectorID)
	featureString, err := a.rdb.Get(ctx, featureKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Features{}, "", errCollectorNotFound
		}
		return Features{}, "", fmt.Errorf("fetch features from redis: %w", err)
	}

	features, err := parseFeatures(featureString)
	if err != nil {
		return Features{}, "", fmt.Errorf("parse features: %w", err)
	}
	return features, "", nil
}

func (a *App) renderAndStoreCollectorConfig(
	ctx context.Context,
	tenantID string,
	collectorID string,
	localAttrs map[string]string,
	requestedHash string,
	forceRefresh bool,
) (string, error) {
	tenantID = resolveTenantID(tenantID)
	collectorID = strings.TrimSpace(collectorID)
	if collectorID == "" {
		return "", fmt.Errorf("collector id is required")
	}

	if !forceRefresh {
		renderedConfig, found, err := a.renderedConfigFromRedis(ctx, tenantID, collectorID)
		if err != nil {
			return "", err
		}
		if found {
			return renderedConfig, nil
		}
	}

	features, customConfig, err := a.featuresForCollector(ctx, tenantID, collectorID)
	if err != nil {
		return "", err
	}

	if customConfig != "" {
		if err := a.storeRenderedConfigInRedis(ctx, tenantID, collectorID, customConfig); err != nil {
			return "", err
		}
		return customConfig, nil
	}

	if a.tmpl == nil {
		return "", fmt.Errorf("template bundle not initialized")
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

	data := map[string]any{
		"TenantID":       tenantID,
		"ID":             collectorID,
		"ComponentID":    sanitizeAlloyIdentifier(collectorID),
		"LocalAttrs":     localAttrs,
		"RequestedHash":  requestedHash,
		"RequestedAtUTC": time.Now().UTC().Format(time.RFC3339),
		"Features":       features,
		"Credentials":    credentials,
		"LGTMCfg":        a.lgtmCfg,
	}

	var buf bytes.Buffer
	if err := a.tmpl.ExecuteTemplate(&buf, configTemplateName, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	renderedConfig := buf.String()
	if err := a.storeRenderedConfigInRedis(ctx, tenantID, collectorID, renderedConfig); err != nil {
		return "", err
	}
	return renderedConfig, nil
}

func (a *App) storeRenderedConfigInRedis(ctx context.Context, tenantID, collectorID, renderedConfig string) error {
	if a.rdb == nil {
		return nil
	}

	if strings.TrimSpace(renderedConfig) == "" {
		return fmt.Errorf("rendered config is empty")
	}

	key := renderedConfigRedisKey(tenantID, collectorID)
	if err := a.rdb.Set(ctx, key, renderedConfig, 0).Err(); err != nil {
		return fmt.Errorf("store rendered config in redis: %w", err)
	}
	return nil
}

func (a *App) renderedConfigFromRedis(ctx context.Context, tenantID, collectorID string) (string, bool, error) {
	if a.rdb == nil {
		return "", false, nil
	}

	key := renderedConfigRedisKey(tenantID, collectorID)
	renderedConfig, err := a.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("fetch rendered config from redis: %w", err)
	}

	renderedConfig = strings.TrimSpace(renderedConfig)
	if renderedConfig == "" {
		return "", false, nil
	}

	return renderedConfig, true, nil
}

func renderedConfigRedisKey(tenantID, collectorID string) string {
	keyTemplate := strings.TrimSpace(os.Getenv("REDIS_RENDERED_CONFIG_KEY_TEMPLATE"))
	if keyTemplate == "" {
		keyTemplate = defaultRenderedConfigKeyTemplate
	}
	return fmt.Sprintf(keyTemplate, resolveTenantID(tenantID), strings.TrimSpace(collectorID))
}

func resolveTenantID(candidate string) string {
	if trimmed := strings.TrimSpace(candidate); trimmed != "" {
		return trimmed
	}
	if fromEnv := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID")); fromEnv != "" {
		return fromEnv
	}
	return defaultTenantID
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

func collectorVersionFromAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}

	preferred := []string{
		"collector.version",
		"collector_version",
		"collectorVersion",
		"alloy.version",
		"alloy_version",
		"alloyVersion",
		"service.version",
		"service_version",
		"serviceVersion",
		"version",
	}
	for _, key := range preferred {
		if value, ok := attrs[key]; ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}

	for key, value := range attrs {
		if !isCollectorVersionKey(key) {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isCollectorVersionKey(key string) bool {
	normalized := collectorVersionKeyNormalizer.Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "collectorversion", "alloyversion", "serviceversion", "version":
		return true
	default:
		return false
	}
}

func hashConfig(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func sanitizeAlloyIdentifier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "_"
	}

	var b strings.Builder
	b.Grow(len(trimmed) + 1)
	lastUnderscore := false

	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	sanitized := strings.Trim(b.String(), "_")
	if sanitized == "" {
		return "_"
	}
	if sanitized[0] >= '0' && sanitized[0] <= '9' {
		return "_" + sanitized
	}
	return sanitized
}
