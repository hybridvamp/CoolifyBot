package coolify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIVersion = "v4"
	defaultCacheTTL   = 30 * time.Second
)

type notFoundError struct {
	message string
}

func (e notFoundError) Error() string {
	return e.message
}

type Client struct {
	BaseURL    string
	Token      string
	APIVersion string
	Client     *http.Client

	Debug bool

	cache    *MemoryCache
	cacheTTL time.Duration

	fallbackVersions []string
}

type ClientOption func(*Client)

func NewClient(baseURL, token string, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		Token:      token,
		APIVersion: defaultAPIVersion,
		Client:     &http.Client{Timeout: 15 * time.Second},
		cache:      NewMemoryCache(),
		cacheTTL:   defaultCacheTTL,
		fallbackVersions: []string{"v4", "v3", "v2", "v1"},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.Client = httpClient
		}
	}
}

func WithAPIVersion(version string) ClientOption {
	return func(c *Client) {
		version = strings.TrimSpace(version)
		if version == "" {
			return
		}
		version = strings.TrimPrefix(version, "/")
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		c.APIVersion = version
	}
}

func WithCacheTTL(ttl time.Duration) ClientOption {
	return func(c *Client) {
		if ttl > 0 {
			c.cacheTTL = ttl
		}
	}
}

func WithCache(cache *MemoryCache) ClientOption {
	return func(c *Client) {
		if cache != nil {
			c.cache = cache
		}
	}
}

func WithDebug(enabled bool) ClientOption {
	return func(c *Client) {
		c.Debug = enabled
	}
}

func (c *Client) apiURL(path string, query url.Values) string {
	base := strings.TrimSuffix(c.BaseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	full := fmt.Sprintf("%s/api/%s%s", base, c.APIVersion, path)
	if len(query) > 0 {
		full = full + "?" + query.Encode()
	}
	return full
}

func (c *Client) apiURLWithVersion(version, path string, query url.Values) string {
	base := strings.TrimSuffix(c.BaseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	full := fmt.Sprintf("%s/api/%s%s", base, version, path)
	if len(query) > 0 {
		full = full + "?" + query.Encode()
	}
	return full
}

func (c *Client) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 15 * time.Second}
	}

	c.authorize(req)
	if c.Debug {
		log.Printf("[coolify] %s %s", req.Method, req.URL.String())
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("unauthenticated: invalid or missing token (401)")
	}
	if resp.StatusCode == http.StatusBadRequest {
		return nil, errors.New("invalid token (400)")
	}
	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return nil, notFoundError{message: fmt.Sprintf("resource not found (404): %s", strings.TrimSpace(string(body)))}
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if c.Debug {
			log.Printf("[coolify] %s %s -> %s\n%s", req.Method, req.URL.String(), resp.Status, strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("unexpected response: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err == nil && c.Debug {
		log.Printf("[coolify] %s %s -> %s", req.Method, req.URL.String(), resp.Status)
	}
	return body, err
}

func (c *Client) versionsToTry() []string {
	seen := make(map[string]struct{})
	var list []string

	primary := c.APIVersion
	if primary == "" {
		primary = defaultAPIVersion
	}
	for _, v := range append([]string{primary}, c.fallbackVersions...) {
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		list = append(list, v)
	}
	return list
}

func (c *Client) doWithFallback(method, path string, query url.Values, body io.Reader) ([]byte, error) {
	versions := c.versionsToTry()
	var lastErr error

	for idx, version := range versions {
		url := c.apiURLWithVersion(version, path, query)
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, err
		}

		respBody, err := c.do(req)
		if err == nil {
			// Cache the working version to avoid future fallbacks.
			c.APIVersion = version
			return respBody, nil
		}

		lastErr = err
		_, isNotFound := err.(notFoundError)
			if !isNotFound || idx == len(versions)-1 {
				return nil, err
			}

		if c.Debug {
			log.Printf("[coolify] received 404 with version %s, trying next version", version)
		}
	}

	return nil, lastErr
}

func decodePage[T any](body []byte) (*Page[T], error) {
	var page Page[T]
	if err := json.Unmarshal(body, &page); err == nil {
		if len(page.Results()) > 0 || page.PageInfo() != (Pagination{}) {
			return &page, nil
		}
	}

	// fallback for array-only responses
	var arr []T
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, err
	}
	page.Data = arr
	return &page, nil
}

func (c *Client) cacheResult(key string, value any) {
	if c.cache == nil {
		return
	}
	c.cache.Set(key, value, c.cacheTTL)
}

func (c *Client) getCached(key string) (any, bool) {
	if c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

func (c *Client) invalidateApplications(uuid string) {
	if c.cache == nil {
		return
	}
	c.cache.DeletePrefix("apps:list:")
	if uuid != "" {
		c.cache.Delete("apps:detail:" + uuid)
	}
}

func listPage[T any](client *Client, path string, query url.Values, cacheKey string) (*Page[T], error) {
	if v, ok := client.getCached(cacheKey); ok {
		if res, ok := v.(*Page[T]); ok {
			return res, nil
		}
	}

	body, err := client.doWithFallback(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}

	page, err := decodePage[T](body)
	if err != nil {
		return nil, err
	}

	if pageInfo := page.PageInfo(); pageInfo.PerPage == 0 && len(query.Get("per_page")) > 0 {
		if perPage, _ := strconv.Atoi(query.Get("per_page")); perPage > 0 {
			p := page.PageInfo()
			p.PerPage = perPage
			page.Pagination = p
		}
	}

	client.cacheResult(cacheKey, page)
	return page, nil
}

func (c *Client) ListApplications(page, perPage int) (*Page[Application], error) {
	query := url.Values{}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	cacheKey := fmt.Sprintf("apps:list:%d:%d", page, perPage)
	return listPage[Application](c, "/applications", query, cacheKey)
}

func (c *Client) GetApplicationByUUID(uuid string) (*ApplicationDetail, error) {
	cacheKey := "apps:detail:" + uuid
	if cached, ok := c.getCached(cacheKey); ok {
		if app, ok := cached.(*ApplicationDetail); ok {
			return app, nil
		}
	}

	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid, nil, nil)
	if err != nil {
		return nil, err
	}

	var app ApplicationDetail
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, err
	}

	c.cacheResult(cacheKey, &app)
	return &app, nil
}

func (c *Client) DeleteApplicationByUUID(uuid string) error {
	if _, err := c.doWithFallback(http.MethodDelete, "/applications/"+uuid, nil, nil); err != nil {
		return err
	}

	c.invalidateApplications(uuid)
	return nil
}

func (c *Client) GetApplicationLogsByUUID(uuid string) (string, error) {
	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid+"/logs", url.Values{"lines": []string{"-1"}}, nil)
	if err != nil {
		return "", err
	}

	var result ApplicationLogs
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return uploadToBatbin(result.Logs)
}

func (c *Client) GetApplicationEnvsByUUID(uuid string) ([]EnvironmentVariable, error) {
	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid+"/envs", nil, nil)
	if err != nil {
		return nil, err
	}

	var envs []EnvironmentVariable
	if err := json.Unmarshal(body, &envs); err != nil {
		return nil, err
	}

	return envs, nil
}

func (c *Client) StartApplicationDeployment(uuid string, force, instantDeploy bool) (*StartDeploymentResponse, error) {
	query := url.Values{}
	if force {
		query.Set("force", "true")
	}
	if instantDeploy {
		query.Set("instant_deploy", "true")
	}

	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid+"/start", query, nil)
	if err != nil {
		return nil, err
	}

	var result StartDeploymentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Deployment kicks off a new state, so bust caches.
	c.invalidateApplications(uuid)
	return &result, nil
}

func (c *Client) StopApplicationByUUID(uuid string) (*StopApplicationResponse, error) {
	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid+"/stop", nil, nil)
	if err != nil {
		return nil, err
	}

	var result StopApplicationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	c.invalidateApplications(uuid)
	return &result, nil
}

func (c *Client) RestartApplicationByUUID(uuid string) (*StartDeploymentResponse, error) {
	body, err := c.doWithFallback(http.MethodGet, "/applications/"+uuid+"/restart", nil, nil)
	if err != nil {
		return nil, err
	}

	var result StartDeploymentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	c.invalidateApplications(uuid)
	return &result, nil
}

func (c *Client) ListDeployments(page, perPage int) (*Page[Deployment], error) {
	query := url.Values{}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	cacheKey := fmt.Sprintf("deployments:list:%d:%d", page, perPage)
	return listPage[Deployment](c, "/deployments", query, cacheKey)
}

func (c *Client) ListDeploymentsByApplication(uuid string, page, perPage int) (*Page[Deployment], error) {
	query := url.Values{}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	cacheKey := fmt.Sprintf("deployments:app:%s:%d:%d", uuid, page, perPage)
	return listPage[Deployment](c, "/applications/"+uuid+"/deployments", query, cacheKey)
}

func (c *Client) ListEnvironments(page, perPage int) (*Page[Environment], error) {
	query := url.Values{}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	cacheKey := fmt.Sprintf("environments:list:%d:%d", page, perPage)
	return listPage[Environment](c, "/environments", query, cacheKey)
}

func (c *Client) ListDatabases(page, perPage int) (*Page[Database], error) {
	query := url.Values{}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	cacheKey := fmt.Sprintf("databases:list:%d:%d", page, perPage)
	return listPage[Database](c, "/databases", query, cacheKey)
}
