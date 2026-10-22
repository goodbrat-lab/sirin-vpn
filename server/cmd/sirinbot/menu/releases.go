package menu

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type ReleaseCache struct {
	mu        sync.RWMutex
	lastFetch time.Time
	urls      map[string]string // "android", "desktop", "windows", "router"
	client    *http.Client
	apiURL    string
}

const (
	defaultAndroidReleaseURL = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/android-v1.3.4"
	defaultDesktopReleaseURL = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/desktop-v1.0.0"
	defaultRouterReleaseURL  = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.2.0"
	releaseCacheTTL          = 1 * time.Hour
)

var defaultReleaseCache = &ReleaseCache{
	urls: map[string]string{
		"android": defaultAndroidReleaseURL,
		"desktop": defaultDesktopReleaseURL,
		"windows": defaultDesktopReleaseURL,
		"router":  defaultRouterReleaseURL,
	},
	client: &http.Client{Timeout: 5 * time.Second},
	apiURL: "https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases",
}

func (c *ReleaseCache) Get(platform string) string {
	c.mu.RLock()
	if time.Since(c.lastFetch) < releaseCacheTTL {
		if u, ok := c.urls[platform]; ok && u != "" {
			c.mu.RUnlock()
			return u
		}
	}
	c.mu.RUnlock()

	c.refresh()

	c.mu.RLock()
	defer c.mu.RUnlock()
	if u, ok := c.urls[platform]; ok && u != "" {
		return u
	}

	switch platform {
	case "android":
		return defaultAndroidReleaseURL
	case "desktop", "windows":
		return defaultDesktopReleaseURL
	case "router":
		return defaultRouterReleaseURL
	default:
		return "https://github.com/goodbrat-lab/sirin-vpn/releases"
	}
}

func (c *ReleaseCache) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.lastFetch) < releaseCacheTTL {
		return
	}

	req, err := http.NewRequest(http.MethodGet, c.apiURL, nil)
	if err != nil {
		c.lastFetch = time.Now()
		return
	}
	req.Header.Set("User-Agent", "sirinbot-release-checker")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.lastFetch = time.Now()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.lastFetch = time.Now()
		return
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		c.lastFetch = time.Now()
		return
	}

	foundAndroid := false
	foundDesktop := false
	foundRouter := false

	for _, r := range releases {
		if !foundAndroid && strings.HasPrefix(r.TagName, "android-") {
			c.urls["android"] = r.HTMLURL
			foundAndroid = true
		}
		if !foundDesktop && strings.HasPrefix(r.TagName, "desktop-") {
			c.urls["desktop"] = r.HTMLURL
			c.urls["windows"] = r.HTMLURL
			foundDesktop = true
		}
		if !foundRouter && strings.HasPrefix(r.TagName, "router-") {
			c.urls["router"] = r.HTMLURL
			foundRouter = true
		}
	}

	c.lastFetch = time.Now()
}

// GetReleaseURL возвращает прямую ссылку на последний релиз платформы.
func (m *Manager) GetReleaseURL(platform string) string {
	return defaultReleaseCache.Get(platform)
}
