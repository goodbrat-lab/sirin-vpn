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
	versions  map[string]string // "android": "1.3.5", etc.
	client    *http.Client
	apiURL    string
}

const (
	defaultAndroidReleaseURL = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/android-v1.3.5"
	defaultDesktopReleaseURL = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/desktop-v1.0.0"
	defaultRouterReleaseURL  = "https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.3.1"
	releaseCacheTTL          = 1 * time.Hour
)

var defaultReleaseCache = &ReleaseCache{
	urls: map[string]string{
		"android": defaultAndroidReleaseURL,
		"desktop": defaultDesktopReleaseURL,
		"windows": defaultDesktopReleaseURL,
		"router":  defaultRouterReleaseURL,
	},
	versions: map[string]string{
		"android": "1.3.5",
		"desktop": "1.0.0",
		"windows": "1.0.0",
		"router":  "1.3.1",
	},
	client: &http.Client{Timeout: 5 * time.Second},
	apiURL: "https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases",
}

func extractVersionFromTag(prefix, tag string) string {
	ver := strings.TrimPrefix(tag, prefix)
	return strings.TrimPrefix(ver, "v")
}

func (c *ReleaseCache) Get(platform string) string {
	_, url := c.GetLatestVersion(platform)
	if url != "" {
		return url
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

func (c *ReleaseCache) GetLatestVersion(platform string) (string, string) {
	c.mu.RLock()
	if time.Since(c.lastFetch) < releaseCacheTTL {
		v, okV := c.versions[platform]
		u, okU := c.urls[platform]
		if okV && okU && v != "" && u != "" {
			c.mu.RUnlock()
			return v, u
		}
	}
	c.mu.RUnlock()

	c.refresh()

	c.mu.RLock()
	defer c.mu.RUnlock()
	v := c.versions[platform]
	u := c.urls[platform]
	if v == "" {
		switch platform {
		case "android":
			v = "1.3.5"
		case "desktop", "windows":
			v = "1.0.0"
		case "router":
			v = "1.3.1"
		}
	}
	if u == "" {
		switch platform {
		case "android":
			u = defaultAndroidReleaseURL
		case "desktop", "windows":
			u = defaultDesktopReleaseURL
		case "router":
			u = defaultRouterReleaseURL
		default:
			u = "https://github.com/goodbrat-lab/sirin-vpn/releases"
		}
	}
	return v, u
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
			c.versions["android"] = extractVersionFromTag("android-", r.TagName)
			foundAndroid = true
		}
		if !foundDesktop && strings.HasPrefix(r.TagName, "desktop-") {
			c.urls["desktop"] = r.HTMLURL
			c.urls["windows"] = r.HTMLURL
			c.versions["desktop"] = extractVersionFromTag("desktop-", r.TagName)
			c.versions["windows"] = extractVersionFromTag("desktop-", r.TagName)
			foundDesktop = true
		}
		if !foundRouter && strings.HasPrefix(r.TagName, "router-") {
			c.urls["router"] = r.HTMLURL
			c.versions["router"] = extractVersionFromTag("router-", r.TagName)
			foundRouter = true
		}
	}

	c.lastFetch = time.Now()
}

// GetReleaseURL возвращает прямую ссылку на последний релиз платформы.
func (m *Manager) GetReleaseURL(platform string) string {
	return defaultReleaseCache.Get(platform)
}

// GetLatestVersion возвращает версию и URL последнего релиза платформы.
func (m *Manager) GetLatestVersion(platform string) (string, string) {
	return defaultReleaseCache.GetLatestVersion(platform)
}
