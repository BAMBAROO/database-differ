package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bryanathallah/db-schema-differ/config"
	"github.com/bryanathallah/db-schema-differ/internal/applier"
	"github.com/bryanathallah/db-schema-differ/internal/connector"
	"github.com/bryanathallah/db-schema-differ/internal/differ"
	"github.com/bryanathallah/db-schema-differ/internal/generator"
	"github.com/bryanathallah/db-schema-differ/internal/introspector"
	"github.com/bryanathallah/db-schema-differ/models"
)

//go:embed assets/index.html
var assetsFS embed.FS

type Server struct {
	cfg *config.Config
}

func NewServer(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

type ConnectionRequest struct {
	Driver    string `json:"driver"`
	SourceDSN string `json:"source_dsn"`
	TargetDSN string `json:"target_dsn"`
}

type GenerateRequest struct {
	Driver    string `json:"driver"`
	SourceDSN string `json:"source_dsn"`
	TargetDSN string `json:"target_dsn"`
	SafeOnly  bool   `json:"safe_only"`
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	// Serve Frontend Single-Page App
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := assetsFS.ReadFile("assets/index.html")
		if err != nil {
			http.Error(w, "Assets Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	// API Endpoint to get default configs
	mux.HandleFunc("/api/defaults", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"Driver":    s.cfg.Driver,
			"SourceDSN": s.cfg.SourceDSN,
			"TargetDSN": s.cfg.TargetDSN,
		})
	})

	// API Endpoint to test database connection credentials
	mux.HandleFunc("/api/test-connection", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req ConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"success":false,"error":"Invalid JSON input"}`))
			return
		}

		conn, err := connector.NewConnector(req.Driver)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Failed to load connector: %v", err),
			})
			return
		}

		// Connect to Source
		srcDB, err := conn.Connect(req.SourceDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Source DSN connection failure: %v", err),
			})
			return
		}
		defer srcDB.Close()

		if err := conn.ValidateReadPrivilege(srcDB); err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Source DB privilege check failure: %v", err),
			})
			return
		}

		// Connect to Target
		tgtDB, err := conn.Connect(req.TargetDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Target DSN connection failure: %v", err),
			})
			return
		}
		defer tgtDB.Close()

		if err := conn.ValidateReadPrivilege(tgtDB); err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Target DB privilege check failure: %v", err),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	// API Endpoint to introspect schemas and calculate diffs
	mux.HandleFunc("/api/diff", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req ConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid JSON input"})
			return
		}

		conn, err := connector.NewConnector(req.Driver)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		srcDB, err := conn.Connect(req.SourceDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Source DB connection failure: %v", err)})
			return
		}
		defer srcDB.Close()

		tgtDB, err := conn.Connect(req.TargetDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Target DB connection failure: %v", err)})
			return
		}
		defer tgtDB.Close()

		var intro introspector.Introspector
		if req.Driver == "mysql" {
			intro = introspector.NewMySQLIntrospector()
		} else {
			intro = introspector.NewPostgresIntrospector()
		}

		srcSchema, err := intro.Introspect(srcDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect source DB: %v", err)})
			return
		}

		tgtSchema, err := intro.Introspect(tgtDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect target DB: %v", err)})
			return
		}

		schemaDiff, err := differ.Diff(srcSchema, tgtSchema)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed calculating diffs: %v", err)})
			return
		}

		type DiffResponse struct {
			Diff   *models.SchemaDiff `json:"diff"`
			Source *models.Schema     `json:"source"`
			Target *models.Schema     `json:"target"`
		}
		_ = json.NewEncoder(w).Encode(DiffResponse{
			Diff:   schemaDiff,
			Source: srcSchema,
			Target: tgtSchema,
		})
	})

	// API Endpoint to generate SQL script
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid JSON input"})
			return
		}

		conn, err := connector.NewConnector(req.Driver)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		srcDB, err := conn.Connect(req.SourceDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Source DB connection failure: %v", err)})
			return
		}
		defer srcDB.Close()

		tgtDB, err := conn.Connect(req.TargetDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Target DB connection failure: %v", err)})
			return
		}
		defer tgtDB.Close()

		var intro introspector.Introspector
		var gen generator.Generator
		if req.Driver == "mysql" {
			intro = introspector.NewMySQLIntrospector()
			gen = generator.NewMySQLGenerator()
		} else {
			intro = introspector.NewPostgresIntrospector()
			gen = generator.NewPostgresGenerator()
		}

		srcSchema, err := intro.Introspect(srcDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect source DB: %v", err)})
			return
		}

		tgtSchema, err := intro.Introspect(tgtDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect target DB: %v", err)})
			return
		}

		schemaDiff, err := differ.Diff(srcSchema, tgtSchema)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed calculating diffs: %v", err)})
			return
		}

		statements, err := gen.Generate(schemaDiff, srcSchema, generator.GenOptions{SafeOnly: req.SafeOnly})
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to generate DDL: %v", err)})
			return
		}

		sqlStr := strings.Join(statements, "\n")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "sql": sqlStr})
	})

	// API Endpoint to apply changes to target database
	mux.HandleFunc("/api/apply", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid JSON" })
			return
		}

		conn, err := connector.NewConnector(req.Driver)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		srcDB, err := conn.Connect(req.SourceDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Source DB connection failure: %v", err)})
			return
		}
		defer srcDB.Close()

		tgtDB, err := conn.Connect(req.TargetDSN)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Target DB connection failure: %v", err)})
			return
		}
		defer tgtDB.Close()

		var intro introspector.Introspector
		var gen generator.Generator
		var applierEngine applier.Applier

		if req.Driver == "mysql" {
			intro = introspector.NewMySQLIntrospector()
			gen = generator.NewMySQLGenerator()
			applierEngine = applier.NewMySQLApplier()
		} else {
			intro = introspector.NewPostgresIntrospector()
			gen = generator.NewPostgresGenerator()
			applierEngine = applier.NewPostgresApplier()
		}

		srcSchema, err := intro.Introspect(srcDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect source DB: %v", err)})
			return
		}

		tgtSchema, err := intro.Introspect(tgtDB, "")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to introspect target DB: %v", err)})
			return
		}

		schemaDiff, err := differ.Diff(srcSchema, tgtSchema)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed calculating diffs: %v", err)})
			return
		}

		statements, err := gen.Generate(schemaDiff, srcSchema, generator.GenOptions{SafeOnly: req.SafeOnly})
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Failed to generate DDL: %v", err)})
			return
		}

		// Apply DDL commands
		err = applierEngine.Apply(tgtDB, statements, "migration_state.json", false)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	return http.ListenAndServe(addr, mux)
}
