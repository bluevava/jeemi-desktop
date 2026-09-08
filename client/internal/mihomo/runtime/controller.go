package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type controllerClient struct {
	httpClient *http.Client
	session    ControllerSession
}

func newControllerClient(httpClient *http.Client, session ControllerSession) *controllerClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}
	return &controllerClient{httpClient: httpClient, session: session}
}

func (c *controllerClient) WaitHealthy(ctx context.Context) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := c.health(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("mihomo control API did not become healthy: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func (c *controllerClient) health(ctx context.Context) error {
	var version struct {
		Version string `json:"version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/version", nil, &version); err != nil {
		return err
	}
	if strings.TrimSpace(version.Version) == "" {
		return fmt.Errorf("mihomo returned an empty version")
	}
	var configuration map[string]any
	return c.doJSON(ctx, http.MethodGet, "/configs", nil, &configuration)
}

func (c *controllerClient) Reload(ctx context.Context, path string) error {
	payload := map[string]string{"path": path}
	return c.doJSON(ctx, http.MethodPut, "/configs?force=true", payload, nil)
}

func (c *controllerClient) SetTUN(ctx context.Context, enabled bool) error {
	return c.doJSON(ctx, http.MethodPatch, "/configs", map[string]any{
		"tun": map[string]bool{"enable": enabled},
	}, nil)
}

func (c *controllerClient) SetMode(ctx context.Context, mode string) error {
	if mode != "rule" && mode != "global" && mode != "direct" {
		return fmt.Errorf("mihomo outbound mode is invalid")
	}
	return c.doJSON(ctx, http.MethodPatch, "/configs", map[string]string{"mode": mode}, nil)
}

func (c *controllerClient) SelectProxy(ctx context.Context, group, proxy string) error {
	group = strings.TrimSpace(group)
	proxy = strings.TrimSpace(proxy)
	if group == "" || proxy == "" {
		return fmt.Errorf("proxy group and proxy names are required")
	}
	return c.doJSON(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), map[string]string{"name": proxy}, nil)
}

func (c *controllerClient) UpdateRuleProvider(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("rule provider name is required")
	}
	return c.doJSON(ctx, http.MethodPut, "/providers/rules/"+url.PathEscape(name), map[string]any{}, nil)
}

func (c *controllerClient) UpdateProxyProvider(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("proxy provider name is required")
	}
	return c.doJSON(ctx, http.MethodPut, "/providers/proxies/"+url.PathEscape(name), map[string]any{}, nil)
}

func (c *controllerClient) doJSON(ctx context.Context, method, path string, input, output any) error {
	return c.doJSONWithLimit(ctx, method, path, input, output, 256<<10)
}

func (c *controllerClient) doJSONWithLimit(ctx context.Context, method, path string, input, output any, limit int64) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.session.BaseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.session.Secret)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("mihomo control request failed: %w", err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return fmt.Errorf("read mihomo control response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mihomo control request returned HTTP %d", response.StatusCode)
	}
	if int64(len(contents)) > limit {
		return fmt.Errorf("mihomo control response exceeds size limit")
	}
	if output != nil && len(bytes.TrimSpace(contents)) > 0 {
		if err := json.Unmarshal(contents, output); err != nil {
			return fmt.Errorf("decode mihomo control response: %w", err)
		}
	}
	return nil
}
