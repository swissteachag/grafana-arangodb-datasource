package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// ArangoDBConfig holds the configuration for ArangoDB connection
type ArangoDBConfig struct {
	URL       string
	Database  string
	Username  string
	Password  string
	Timeout   int
	BatchSize int // Maximum number of documents per batch
}

// ArangoDBClient handles communication with ArangoDB
type ArangoDBClient struct {
	config       *ArangoDBConfig
	httpClient   *http.Client
	baseURL      string
	jwtToken     string
	tokenExpiry  time.Time
}

// AuthRequest represents a JWT authentication request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents a JWT authentication response
type AuthResponse struct {
	JWT string `json:"jwt"`
}

// QueryResult represents the result of an AQL query
type QueryResult struct {
	Result []interface{} `json:"result"`
	HasMore bool         `json:"hasMore"`
	Count   int          `json:"count"`
	ID      string       `json:"id,omitempty"` // Cursor ID for pagination
	Extra   struct {
		Stats struct {
			ExecutionTime float64 `json:"executionTime"`
			Scanned       int     `json:"scannedFull"`
			Filtered      int     `json:"filtered"`
		} `json:"stats"`
	} `json:"extra"`
}

// Collection represents an ArangoDB collection
type Collection struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       int    `json:"type"` // 2 = document, 3 = edge
	Status     int    `json:"status"`
	IsSystem   bool   `json:"isSystem"`
}

// NewArangoDBClient creates a new ArangoDB client
func NewArangoDBClient(config *ArangoDBConfig) (*ArangoDBClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("ArangoDB URL is required")
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &ArangoDBClient{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: fmt.Sprintf("%s/_db/%s", config.URL, config.Database),
	}

	// Authenticate and get JWT token if credentials are provided
	if config.Username != "" && config.Password != "" {
		if err := client.authenticate(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	return client, nil
}

// authenticate obtains a JWT token from ArangoDB
func (c *ArangoDBClient) authenticate(ctx context.Context) error {
	authURL := c.config.URL + "/_open/auth"
	
	authReq := AuthRequest{
		Username: c.config.Username,
		Password: c.config.Password,
	}

	jsonData, err := json.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate with ArangoDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.jwtToken = authResp.JWT
	// JWT tokens expire in 30 minutes, so set expiry time to 25 minutes from now to refresh early
	c.tokenExpiry = time.Now().Add(25 * time.Minute)
	log.DefaultLogger.Debug("Successfully authenticated with ArangoDB", "tokenExpiry", c.tokenExpiry.Format(time.RFC3339))
	return nil
}

// ensureAuthenticated checks if we have a valid JWT token and refreshes if needed
func (c *ArangoDBClient) ensureAuthenticated(ctx context.Context) error {
	// If no credentials provided, no authentication needed
	if c.config.Username == "" || c.config.Password == "" {
		return nil
	}

	// Check if we need to authenticate or refresh token
	now := time.Now()
	needsAuth := c.jwtToken == "" || now.After(c.tokenExpiry)

	if needsAuth {
		log.DefaultLogger.Debug("Token expired or missing, re-authenticating", "tokenExpiry", c.tokenExpiry.Format(time.RFC3339), "now", now.Format(time.RFC3339))
		return c.authenticate(ctx)
	}

	return nil
}

// ExecuteQuery executes an AQL query with automatic token refresh
func (c *ArangoDBClient) ExecuteQuery(ctx context.Context, query string, bindVars map[string]interface{}) (*QueryResult, error) {
	return c.executeQueryWithRetry(ctx, query, bindVars, false)
}

// executeQueryWithRetry executes an AQL query with optional retry on auth failure
func (c *ArangoDBClient) executeQueryWithRetry(ctx context.Context, query string, bindVars map[string]interface{}, isRetry bool) (*QueryResult, error) {
	// Ensure we have a valid authentication token
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get batch size from config, default to 50000 if not set
	batchSize := c.config.BatchSize
	if batchSize <= 0 {
		batchSize = 50000
	}

	log.DefaultLogger.Debug("Executing query with batch size", "batchSize", batchSize)

	requestBody := map[string]interface{}{
		"query":    query,
		"bindVars": bindVars,
		"options": map[string]interface{}{
			"fullCount": true,
			"maxPlans":  1,
			"batchSize": batchSize, // Use configurable batch size
			"count":     true,      // Include count in response
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/_api/cursor", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.jwtToken != "" {
		req.Header.Set("Authorization", "bearer "+c.jwtToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle authentication failure with retry
	if resp.StatusCode == 401 && !isRetry {
		log.DefaultLogger.Debug("Received 401, attempting to re-authenticate and retry")
		// Force re-authentication by clearing token
		c.jwtToken = ""
		c.tokenExpiry = time.Time{}
		return c.executeQueryWithRetry(ctx, query, bindVars, true)
	}

	if resp.StatusCode >= 400 {
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if errorMsg, ok := errorResp["errorMessage"].(string); ok {
				return nil, fmt.Errorf("ArangoDB error: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("ArangoDB request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	log.DefaultLogger.Debug("Query result details", "resultCount", len(result.Result), "hasMore", result.HasMore, "cursorId", result.ID, "count", result.Count)

	// Handle cursor pagination if there are more results
	if result.HasMore && result.ID != "" {
		log.DefaultLogger.Debug("Query has more results, fetching via cursor", "cursorId", result.ID, "currentCount", len(result.Result))
		
		// Fetch remaining results using cursor
		moreResults, err := c.fetchCursorResults(ctx, result.ID, isRetry)
		if err != nil {
			log.DefaultLogger.Warn("Failed to fetch cursor results", "error", err)
			// Return what we have so far rather than failing completely
		} else {
			// Append additional results
			result.Result = append(result.Result, moreResults...)
			log.DefaultLogger.Debug("Fetched additional results via cursor", "totalResults", len(result.Result))
		}
	}

	return &result, nil
}

// fetchCursorResults fetches remaining results using cursor pagination
func (c *ArangoDBClient) fetchCursorResults(ctx context.Context, cursorID string, isRetry bool) ([]interface{}, error) {
	var allResults []interface{}
	
	for {
		req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/_api/cursor/"+cursorID, nil)
		if err != nil {
			return allResults, fmt.Errorf("failed to create cursor request: %w", err)
		}

		if c.jwtToken != "" {
			req.Header.Set("Authorization", "bearer "+c.jwtToken)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return allResults, fmt.Errorf("failed to execute cursor request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return allResults, fmt.Errorf("failed to read cursor response: %w", err)
		}

		// Handle authentication failure with retry
		if resp.StatusCode == 401 && !isRetry {
			log.DefaultLogger.Debug("Received 401 in cursor fetch, attempting to re-authenticate")
			// Force re-authentication by clearing token
			c.jwtToken = ""
			c.tokenExpiry = time.Time{}
			// Note: We can't easily retry cursor requests after re-auth since the cursor might be invalid
			return allResults, fmt.Errorf("authentication failed during cursor fetch")
		}

		if resp.StatusCode >= 400 {
			var errorResp map[string]interface{}
			if json.Unmarshal(body, &errorResp) == nil {
				if errorMsg, ok := errorResp["errorMessage"].(string); ok {
					return allResults, fmt.Errorf("ArangoDB cursor error: %s", errorMsg)
				}
			}
			return allResults, fmt.Errorf("ArangoDB cursor request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var cursorResult QueryResult
		if err := json.Unmarshal(body, &cursorResult); err != nil {
			return allResults, fmt.Errorf("failed to unmarshal cursor response: %w", err)
		}

		// Append results from this batch
		allResults = append(allResults, cursorResult.Result...)
		
		// If no more results, we're done
		if !cursorResult.HasMore {
			break
		}
		
		// Update cursor ID for next iteration
		cursorID = cursorResult.ID
		
		// Safety check to prevent infinite loops
		if len(allResults) > 100000 {
			log.DefaultLogger.Warn("Cursor fetch reached safety limit", "totalResults", len(allResults))
			break
		}
	}
	
	return allResults, nil
}

// GetCollections retrieves all collections from the database with automatic token refresh
func (c *ArangoDBClient) GetCollections(ctx context.Context) ([]Collection, error) {
	return c.getCollectionsWithRetry(ctx, false)
}

// getCollectionsWithRetry retrieves collections with optional retry on auth failure
func (c *ArangoDBClient) getCollectionsWithRetry(ctx context.Context, isRetry bool) ([]Collection, error) {
	// Ensure we have a valid authentication token
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/_api/collection", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.jwtToken != "" {
		req.Header.Set("Authorization", "bearer "+c.jwtToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Handle authentication failure with retry
	if resp.StatusCode == 401 && !isRetry {
		log.DefaultLogger.Debug("Received 401 in GetCollections, attempting to re-authenticate and retry")
		// Force re-authentication by clearing token
		c.jwtToken = ""
		c.tokenExpiry = time.Time{}
		return c.getCollectionsWithRetry(ctx, true)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to get collections: status %d", resp.StatusCode)
	}

	var response struct {
		Result []Collection `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter out system collections
	var userCollections []Collection
	for _, col := range response.Result {
		if !col.IsSystem && !strings.HasPrefix(col.Name, "_") {
			userCollections = append(userCollections, col)
		}
	}

	return userCollections, nil
}

// GetCollectionFields retrieves field names from a collection by sampling documents
func (c *ArangoDBClient) GetCollectionFields(ctx context.Context, collection string) ([]string, error) {
	query := fmt.Sprintf("FOR doc IN %s LIMIT 10 RETURN ATTRIBUTES(doc)", collection)
	
	result, err := c.ExecuteQuery(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	fieldsSet := make(map[string]bool)
	for _, item := range result.Result {
		if fields, ok := item.([]interface{}); ok {
			for _, field := range fields {
				if fieldName, ok := field.(string); ok {
					fieldsSet[fieldName] = true
				}
			}
		}
	}

	var fields []string
	for field := range fieldsSet {
		fields = append(fields, field)
	}

	return fields, nil
}

// Ping checks if the ArangoDB server is reachable with automatic token refresh
func (c *ArangoDBClient) Ping(ctx context.Context) error {
	return c.pingWithRetry(ctx, false)
}

// pingWithRetry checks connectivity with optional retry on auth failure
func (c *ArangoDBClient) pingWithRetry(ctx context.Context, isRetry bool) error {
	log.DefaultLogger.Info("Attempting to ping ArangoDB", "url", c.config.URL+"/_api/version", "username", c.config.Username)
	
	// Ensure we have a valid authentication token
	if err := c.ensureAuthenticated(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL+"/_api/version", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	if c.jwtToken != "" {
		log.DefaultLogger.Debug("Setting JWT token for ping request")
		req.Header.Set("Authorization", "bearer "+c.jwtToken)
	} else {
		log.DefaultLogger.Debug("No JWT token available")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping ArangoDB: %w", err)
	}
	defer resp.Body.Close()

	// Handle authentication failure with retry
	if resp.StatusCode == 401 && !isRetry {
		log.DefaultLogger.Debug("Received 401 in Ping, attempting to re-authenticate and retry")
		// Force re-authentication by clearing token
		c.jwtToken = ""
		c.tokenExpiry = time.Time{}
		return c.pingWithRetry(ctx, true)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.DefaultLogger.Error("Ping failed", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("ping failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.DefaultLogger.Debug("Successfully pinged ArangoDB", "status", resp.StatusCode)
	return nil
}

// Close closes the HTTP client
func (c *ArangoDBClient) Close() {
	// Nothing to close for HTTP client
}
