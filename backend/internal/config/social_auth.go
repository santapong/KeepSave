package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type SocialProvider struct {
	ClientID     string
	ClientSecret string
}
type SocialAuth struct {
	Origin string
	GitHub SocialProvider
	Google SocialProvider
}

func loadSocialAuth(env string) (SocialAuth, error) {
	c := SocialAuth{Origin: strings.TrimRight(os.Getenv("SOCIAL_AUTH_ORIGIN"), "/"),
		GitHub: SocialProvider{os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET")},
		Google: SocialProvider{os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")}}
	enabled := false
	for _, p := range []SocialProvider{c.GitHub, c.Google} {
		if (p.ClientID == "") != (p.ClientSecret == "") {
			return c, fmt.Errorf("social login requires both client ID and client secret for each enabled provider")
		}
		enabled = enabled || p.ClientID != ""
	}
	if !enabled && c.Origin == "" {
		return c, nil
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return c, fmt.Errorf("SOCIAL_AUTH_ORIGIN must be the frontend origin without a path, query or fragment")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local && env != "production") {
		return c, fmt.Errorf("SOCIAL_AUTH_ORIGIN requires HTTPS (HTTP loopback is allowed only in development)")
	}
	return c, nil
}
