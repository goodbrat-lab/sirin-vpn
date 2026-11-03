package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type APIClient struct {
	baseURL    string
	adminToken string
	http       *http.Client
}

func NewAPIClient(baseURL, adminToken string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		adminToken: adminToken,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

type AdminCreateRequest struct {
	Name      string `json:"name"`
	DaysValid int    `json:"days_valid"`
}

type AdminCreateResponse struct {
	Client struct {
		UUID      string `json:"uuid"`
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
	} `json:"client"`
	Key string `json:"key"`
}

func (c *APIClient) CreateClient(name string, daysValid int) (*AdminCreateResponse, error) {
	body, _ := json.Marshal(AdminCreateRequest{Name: name, DaysValid: daysValid})
	req, _ := http.NewRequest("POST", c.baseURL+"/v1/admin/clients", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}
	var out AdminCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) Health() (map[string]interface{}, error) {
	resp, err := c.http.Get(c.baseURL + "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

type MonitorReportItem struct {
	Node      string    `json:"node"`
	RTTMs     float64   `json:"rtt_ms"`
	LossPct   float64   `json:"loss_pct"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AdminMonitorResponse struct {
	Contract string                       `json:"contract"`
	Status   string                       `json:"status"`
	Reports  map[string]MonitorReportItem `json:"reports"`
}

func (c *APIClient) GetMonitorReports() (map[string]MonitorReportItem, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("api client unconfigured")
	}
	req, err := http.NewRequest("GET", c.baseURL+"/v1/admin/monitor", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}
	var out AdminMonitorResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Reports, nil
}

