package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

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

// registerRoutes takes a *http.ServeMux and registers some HTTP handlers.
func (a *App) registerRoutes(mux *http.ServeMux) {
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
