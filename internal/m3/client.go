package m3

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

type Client struct {
	target     Target
	baseURL    *url.URL
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
}

func NewClient(target Target) (*Client, error) {
	parsed, err := url.Parse(target.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base url must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: target.InsecureSkipVerify} //nolint:gosec // Explicit per-target compatibility option.

	return &Client{
		target:  target,
		baseURL: parsed,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}, nil
}

func (c *Client) Target() Target {
	return c.target
}

func (c *Client) Authenticate(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{
		"username": c.target.Username,
		"password": c.target.Password,
	})
	if err != nil {
		return fmt.Errorf("marshal auth request: %w", err)
	}

	var token tokenResponse
	if err := c.doJSON(ctx, http.MethodPost, c.apiPath("oauth2/token/"), bytes.NewReader(body), false, &token); err != nil {
		return fmt.Errorf("authenticate target %q: %w", c.target.Name, err)
	}
	if token.AccessToken == "" {
		return fmt.Errorf("authenticate target %q: empty access token", c.target.Name)
	}

	c.mu.Lock()
	c.accessToken = token.AccessToken
	c.mu.Unlock()
	return nil
}

func (c *Client) FetchSnapshot(ctx context.Context) (Snapshot, error) {
	snapshot := Snapshot{TargetName: c.target.Name, BaseURL: c.target.BaseURL}

	if err := c.get(ctx, c.apiPath("powerService/status"), &snapshot.PowerServiceStatus); err != nil {
		return snapshot, err
	}
	if err := c.get(ctx, c.apiPath("environmentService/status"), &snapshot.EnvironmentServiceStatus); err != nil {
		return snapshot, err
	}
	if err := c.get(ctx, c.apiPath("alarmService/parametricCount"), &snapshot.AlarmCounts); err != nil {
		return snapshot, err
	}
	if err := c.get(ctx, c.apiPath("alarmService/mostCriticalAlarm"), &snapshot.MostCriticalAlarm); err != nil {
		return snapshot, err
	}

	powerIDs, err := c.memberIDs(ctx, c.apiPath("powerDistributions"))
	if err != nil {
		return snapshot, err
	}
	for _, id := range powerIDs {
		pd, err := c.fetchPowerDistribution(ctx, id)
		if err != nil {
			return snapshot, err
		}
		snapshot.PowerDistributions = append(snapshot.PowerDistributions, pd)
	}

	supplierIDs, err := c.memberIDs(ctx, c.apiPath("powerService/suppliers"))
	if err != nil {
		return snapshot, err
	}
	for _, id := range supplierIDs {
		supplier, err := c.fetchSupplier(ctx, id)
		if err != nil {
			return snapshot, err
		}
		snapshot.Suppliers = append(snapshot.Suppliers, supplier)
	}

	if snapshot.Temperatures, err = c.fetchEnvironmentSensors(ctx, "temperatures"); err != nil {
		return snapshot, err
	}
	if snapshot.Humidities, err = c.fetchEnvironmentSensors(ctx, "humidities"); err != nil {
		return snapshot, err
	}
	if snapshot.EnvironmentInputs, err = c.fetchEnvironmentInputs(ctx); err != nil {
		return snapshot, err
	}

	return snapshot, nil
}

func (c *Client) fetchPowerDistribution(ctx context.Context, id string) (PowerDistribution, error) {
	pd := PowerDistribution{ID: lastPathSegment(id)}
	base := c.resourcePath(id)
	if err := c.get(ctx, path.Join(base, "identification"), &pd.Identification); err != nil {
		return pd, err
	}
	if err := c.get(ctx, path.Join(base, "status"), &pd.Status); err != nil {
		return pd, err
	}
	if err := c.get(ctx, path.Join(base, "backupSystem/powerBank/status"), &pd.PowerBank); err != nil {
		return pd, err
	}

	var err error
	if pd.Inputs, err = c.fetchPowerResources(ctx, path.Join(base, "inputs"), "input"); err != nil {
		return pd, err
	}
	if pd.Outputs, err = c.fetchPowerResources(ctx, path.Join(base, "outputs"), "output"); err != nil {
		return pd, err
	}
	if pd.Outlets, err = c.fetchPowerResources(ctx, path.Join(base, "outlets"), "outlet"); err != nil {
		return pd, err
	}
	return pd, nil
}

func (c *Client) fetchPowerResources(ctx context.Context, collectionPath string, kind string) ([]PowerResource, error) {
	ids, err := c.memberIDs(ctx, collectionPath)
	if err != nil {
		return nil, err
	}

	resources := make([]PowerResource, 0, len(ids))
	for _, id := range ids {
		resource := PowerResource{ID: lastPathSegment(id), Kind: kind}
		base := c.resourcePath(id)
		if err := c.get(ctx, path.Join(base, "identification"), &resource.Identification); err != nil {
			return nil, err
		}
		if err := c.get(ctx, path.Join(base, "status"), &resource.Status); err != nil {
			return nil, err
		}
		if err := c.get(ctx, path.Join(base, "measures"), &resource.Measures); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (c *Client) fetchSupplier(ctx context.Context, id string) (Supplier, error) {
	supplier := Supplier{ID: lastPathSegment(id)}
	base := c.resourcePath(id)
	if err := c.get(ctx, path.Join(base, "identification"), &supplier.Identification); err != nil {
		// Supplier identity is not consistently exposed by older card firmware.
		supplier.Identification = Identification{}
	}
	if err := c.get(ctx, path.Join(base, "measures"), &supplier.Measures); err != nil {
		return supplier, err
	}
	if err := c.get(ctx, path.Join(base, "summary"), &supplier.Summary); err != nil {
		return supplier, err
	}
	return supplier, nil
}

func (c *Client) fetchEnvironmentSensors(ctx context.Context, kind string) ([]EnvironmentSensor, error) {
	ids, err := c.memberIDs(ctx, c.apiPath("environmentService/"+kind))
	if err != nil {
		return nil, err
	}

	sensors := make([]EnvironmentSensor, 0, len(ids))
	for _, id := range ids {
		var sensor EnvironmentSensor
		if err := c.get(ctx, c.resourcePath(id), &sensor); err != nil {
			return nil, err
		}
		sensors = append(sensors, sensor)
	}
	return sensors, nil
}

func (c *Client) fetchEnvironmentInputs(ctx context.Context) ([]EnvironmentInput, error) {
	ids, err := c.memberIDs(ctx, c.apiPath("environmentService/inputs"))
	if err != nil {
		return nil, err
	}

	inputs := make([]EnvironmentInput, 0, len(ids))
	for _, id := range ids {
		var input EnvironmentInput
		if err := c.get(ctx, c.resourcePath(id), &input); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func (c *Client) memberIDs(ctx context.Context, requestPath string) ([]string, error) {
	var list memberList
	if err := c.get(ctx, requestPath, &list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list.Members))
	for _, member := range list.Members {
		if member.ID != "" {
			ids = append(ids, member.ID)
		}
	}
	return ids, nil
}

func (c *Client) get(ctx context.Context, requestPath string, out any) error {
	if err := c.doJSON(ctx, http.MethodGet, requestPath, nil, true, out); err == nil {
		return nil
	} else if !isUnauthorized(err) {
		return err
	}

	if err := c.Authenticate(ctx); err != nil {
		return err
	}
	if err := c.doJSON(ctx, http.MethodGet, requestPath, nil, true, out); err != nil {
		return err
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, method string, requestPath string, body io.Reader, auth bool, out any) error {
	requestURL := c.urlFor(requestPath)
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return fmt.Errorf("create request %s %s: %w", method, requestURL, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		token := c.token()
		if token == "" {
			if err := c.Authenticate(ctx); err != nil {
				return err
			}
			token = c.token()
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", requestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return unauthorizedError{status: resp.Status}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("request %s returned %s and body read failed: %w", requestURL, resp.Status, readErr)
		}
		return fmt.Errorf("request %s returned %s: %s", requestURL, resp.Status, strings.TrimSpace(string(limited)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response %s: %w", requestURL, err)
	}
	return nil
}

func (c *Client) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken
}

func (c *Client) apiPath(suffix string) string {
	return "/rest/mbdetnrs/" + c.target.APIVersion + "/" + strings.TrimLeft(suffix, "/")
}

func (c *Client) resourcePath(id string) string {
	if strings.HasPrefix(id, "/rest/") {
		return id
	}
	return c.apiPath(id)
}

func (c *Client) urlFor(requestPath string) string {
	copyURL := *c.baseURL
	copyURL.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(requestPath, "/")
	return copyURL.String()
}

func lastPathSegment(id string) string {
	trimmed := strings.TrimRight(id, "/")
	return path.Base(trimmed)
}

type unauthorizedError struct {
	status string
}

func (e unauthorizedError) Error() string {
	return "unauthorized: " + e.status
}

func isUnauthorized(err error) bool {
	_, ok := err.(unauthorizedError)
	return ok
}
