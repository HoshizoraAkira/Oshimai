package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

const maxAutoDiscoverEndpoints = 15

var hrefRegex = regexp.MustCompile(`(?i)href=["']([^"'#][^"']*)["']`)

type sitemapURLSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// AutoDiscover implements the "1-click" onboarding path: given nothing but a homepage URL, it
// crawls robots.txt, sitemap.xml, and the homepage's own links to build a same-origin endpoint
// map — no OpenAPI spec, no OTel trace, no manual recording required. Every discovered page
// becomes a sequential GET-only Step; this favors breadth of coverage over request realism, which
// is the right trade-off for a first "does my site survive load at all" smoke test.
func AutoDiscover(ctx context.Context, targetURL string, cfg GeneratorConfig) (*vusession.Scenario, error) {
	base, err := url.Parse(targetURL)
	if err != nil || base.Host == "" {
		return nil, fmt.Errorf("invalid target URL %q", targetURL)
	}
	origin := base.Scheme + "://" + base.Host

	client := &http.Client{Timeout: 10 * time.Second}
	paths := map[string]bool{base.Path: true}
	if base.Path == "" {
		paths["/"] = true
	}

	// 1. robots.txt -> Sitemap: directives.
	var sitemapURLs []string
	if body, ok := fetch(ctx, client, origin+"/robots.txt"); ok {
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
				sitemapURLs = append(sitemapURLs, strings.TrimSpace(line[len("sitemap:"):]))
			}
		}
	}
	if len(sitemapURLs) == 0 {
		sitemapURLs = []string{origin + "/sitemap.xml"}
	}

	// 2. sitemap.xml -> <loc> entries.
	for _, smURL := range sitemapURLs {
		if body, ok := fetch(ctx, client, smURL); ok {
			var set sitemapURLSet
			if err := xml.Unmarshal([]byte(body), &set); err == nil {
				for _, u := range set.URLs {
					if p, sameOrigin := samOriginPath(u.Loc, origin); sameOrigin {
						paths[p] = true
					}
					if len(paths) >= maxAutoDiscoverEndpoints {
						break
					}
				}
			}
		}
	}

	// 3. Homepage link scrape (fallback / supplement when no sitemap exists).
	if len(paths) < maxAutoDiscoverEndpoints {
		if body, ok := fetch(ctx, client, origin+base.Path); ok {
			for _, match := range hrefRegex.FindAllStringSubmatch(body, -1) {
				if len(paths) >= maxAutoDiscoverEndpoints {
					break
				}
				href := match[1]
				if strings.HasPrefix(href, "//") || strings.HasPrefix(href, "http") {
					if p, sameOrigin := samOriginPath(href, origin); sameOrigin {
						paths[p] = true
					}
					continue
				}
				if strings.HasPrefix(href, "/") {
					paths[href] = true
				}
			}
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("could not discover any pages on %s (robots.txt, sitemap.xml, and homepage links all empty/unreachable)", origin)
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_autodiscover_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "1-Click Auto-Discovered Scenario"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	i := 0
	for p := range paths {
		if i >= maxAutoDiscoverEndpoints {
			break
		}
		i++
		stepID := fmt.Sprintf("page_%d", i)
		steps[stepID] = &vusession.Step{
			ID:      stepID,
			Name:    fmt.Sprintf("GET %s", p),
			Request: vusession.RequestConfig{Method: "GET", Path: p, Timeout: vusession.Duration(cfg.DefaultTimeout)},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 200, MaxCode: 399},
			},
			OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
		}
		order = append(order, stepID)
	}

	for idx, id := range order {
		if idx+1 < len(order) {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: order[idx+1], Probability: 1.0}}
		} else {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: "END", Probability: 1.0}}
		}
	}

	sc := &vusession.Scenario{
		ID:             cfg.ScenarioID,
		Name:           cfg.ScenarioName,
		Description:    fmt.Sprintf("Auto-discovered %d pages from robots.txt/sitemap.xml/homepage links — no spec required.", len(order)),
		BaseURL:        origin,
		InitialStepID:  order[0],
		DefaultHeaders: cfg.DefaultHeaders,
		Timeout:        vusession.Duration(cfg.DefaultTimeout),
		Steps:          steps,
	}

	if err := vusession.ValidateScenario(sc); err != nil {
		return nil, fmt.Errorf("auto-discovered scenario failed validation: %w", err)
	}
	return sc, nil
}

func fetch(ctx context.Context, client *http.Client, target string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "OshimaiAutoDiscover/1.0 (+load-test onboarding crawler)")
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // Cap at 2MB.
	if err != nil {
		return "", false
	}
	return string(body), true
}

func samOriginPath(rawURL, origin string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	if u.Scheme+"://"+u.Host != origin {
		return "", false
	}
	if u.Path == "" {
		return "/", true
	}
	return u.Path, true
}
