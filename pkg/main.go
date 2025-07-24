package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Make sure ArangoDBDatasource implements required interfaces
var (
	_ backend.QueryDataHandler      = (*ArangoDBDatasource)(nil)
	_ backend.CheckHealthHandler    = (*ArangoDBDatasource)(nil)
	_ instancemgmt.InstanceDisposer = (*ArangoDBDatasource)(nil)
)

// ArangoDBDatasource is the main datasource struct
type ArangoDBDatasource struct {
	client *ArangoDBClient
}

// NewArangoDBDatasource creates a new datasource instance
func NewArangoDBDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info("Creating new ArangoDB datasource instance", "datasourceUID", settings.UID, "datasourceName", settings.Name)

	var jsonData map[string]interface{}
	if err := json.Unmarshal(settings.JSONData, &jsonData); err != nil {
		log.DefaultLogger.Error("Failed to unmarshal JSON data", "error", err)
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	config := &ArangoDBConfig{
		URL:       getString(jsonData, "url", "http://localhost:8529"),
		Database:  getString(jsonData, "database", "_system"),
		Username:  getString(jsonData, "username", ""),
		Password:  settings.DecryptedSecureJSONData["password"],
		Timeout:   getInt(jsonData, "timeout", 30),
		BatchSize: getInt(jsonData, "batchSize", 50000),
	}

	log.DefaultLogger.Info("ArangoDB config", "url", config.URL, "database", config.Database, "username", config.Username, "batchSize", config.BatchSize, "hasPassword", config.Password != "")

	client, err := NewArangoDBClient(config)
	if err != nil {
		log.DefaultLogger.Error("Failed to create ArangoDB client", "error", err)
		return nil, fmt.Errorf("failed to create ArangoDB client: %w", err)
	}

	log.DefaultLogger.Info("ArangoDB datasource instance created successfully")
	return &ArangoDBDatasource{
		client: client,
	}, nil
}

// QueryData handles data queries
func (d *ArangoDBDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData called", "queries", len(req.Queries))

	response := backend.NewQueryDataResponse()

	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// query executes a single query
func (d *ArangoDBDatasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var qm QueryModel
	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	log.DefaultLogger.Debug("Query model received", "queryType", qm.QueryType, "collection", qm.Collection, "limit", qm.Limit, "filter", qm.Filter)

	// Execute the query based on type
	var aqlQuery string
	var err error

	if qm.QueryType == "aql" && qm.AQLQuery != "" {
		// Use custom AQL query
		log.DefaultLogger.Debug("Processing custom AQL query", "originalQuery", qm.AQLQuery)
		aqlQuery = d.interpolateVariables(qm.AQLQuery, query.TimeRange.From.UnixMilli(), query.TimeRange.To.UnixMilli())
		log.DefaultLogger.Debug("Processed AQL query", "processedQuery", aqlQuery)
	} else {
		// Build collection query
		aqlQuery, err = d.buildCollectionQuery(&qm, query.TimeRange.From.UnixMilli(), query.TimeRange.To.UnixMilli())
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to build query: %v", err.Error()))
		}
	}

	log.DefaultLogger.Debug("Executing AQL query", "query", aqlQuery)

	// Only pass bind parameters if they are used in the query
	bindVars := map[string]interface{}{}
	if strings.Contains(aqlQuery, "@from") {
		bindVars["from"] = query.TimeRange.From.UnixMilli()
	}
	if strings.Contains(aqlQuery, "@to") {
		bindVars["to"] = query.TimeRange.To.UnixMilli()
	}

	log.DefaultLogger.Debug("Using bind parameters", "bindVars", bindVars)

	result, err := d.client.ExecuteQuery(ctx, aqlQuery, bindVars)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to execute query: %v", err.Error()))
	}

	// Transform result to Grafana data frames
	frames, err := d.transformResult(result, &qm, query.RefID)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to transform result: %v", err.Error()))
	}

	return backend.DataResponse{
		Frames: frames,
	}
}

// CheckHealth implements health check
func (d *ArangoDBDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("CheckHealth called - backend health check invoked")

	status := backend.HealthStatusOk
	message := "Successfully connected to ArangoDB"

	if err := d.client.Ping(ctx); err != nil {
		status = backend.HealthStatusError
		message = fmt.Sprintf("Failed to connect to ArangoDB: %v", err)
		log.DefaultLogger.Error("Health check failed", "error", err)
	} else {
		log.DefaultLogger.Info("Health check successful")
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

// Dispose cleans up datasource resources
func (d *ArangoDBDatasource) Dispose() {
	if d.client != nil {
		d.client.Close()
	}
}

func main() {
	log.DefaultLogger.Info("ArangoDB Plugin starting up...", "version", "1.0.0", "args", os.Args)
	
	// Handle command line arguments for debugging
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Println("ArangoDB Grafana Plugin v1.0.0")
			fmt.Println("Usage: gpx_arangodb-datasource")
			os.Exit(0)
		case "--version", "-v":
			fmt.Println("1.0.0")
			os.Exit(0)
		}
	}
	
	log.DefaultLogger.Info("Starting datasource management...")
	
	// Start listening to requests sent from Grafana
	if err := datasource.Manage("arangodb-datasource", func(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		log.DefaultLogger.Info("Grafana is requesting new datasource instance", "uid", settings.UID, "name", settings.Name, "type", settings.Type)
		return NewArangoDBDatasource(settings)
	}, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error("Failed to start datasource", "error", err.Error())
		os.Exit(1)
	}
}

// Helper functions
func getString(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getInt(data map[string]interface{}, key string, defaultValue int) int {
	if val, ok := data[key]; ok {
		if num, ok := val.(float64); ok {
			return int(num)
		}
	}
	return defaultValue
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
