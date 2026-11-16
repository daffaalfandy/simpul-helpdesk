package connector

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
)

// Location is where a repository lives, read from its web address.
type Location struct {
	Host     string // lower-case host, with the port when there is one
	FullName string // owner/name on GitHub; group/…/project on GitLab
	WebURL   string // scheme://host/full_name
}

var errRepositoryURL = errors.New("enter the repository's web address, e.g. https://github.com/owner/name")

const (
	schemeHTTPS     = "https"
	schemeHTTP      = "http" // self-managed GitLab inside a private network
	gitSuffix       = ".git"
	gitLabRouteMark = "/-/" // GitLab pages under a project: /group/project/-/tree/main
	pathSeparator   = "/"
	// gitHubSegments is owner/name; GitLab nests a project in one or more groups.
	gitHubSegments    = 2
	minGitLabSegments = 2
	maxURLLen         = 600
)

// segmentPattern is what GitHub and GitLab allow in owner, group and repository names.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ParseLocation reads a repository's web address as the admin pasted it: a trailing slash, a
// ".git" ending or a GitLab page below the project are dropped. Credentials, queries and other
// schemes are refused.
func ParseLocation(p gitevent.Provider, raw string) (Location, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > maxURLLen || !p.Valid() {
		return Location{}, errRepositoryURL
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != schemeHTTPS && u.Scheme != schemeHTTP) || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" {
		return Location{}, errRepositoryURL
	}
	path := u.Path
	if p == gitevent.ProviderGitLab {
		path, _, _ = strings.Cut(path, gitLabRouteMark)
	}
	path = strings.TrimSuffix(strings.Trim(path, pathSeparator), gitSuffix)
	segments := strings.Split(path, pathSeparator)
	switch {
	case p == gitevent.ProviderGitHub && len(segments) != gitHubSegments,
		p == gitevent.ProviderGitLab && len(segments) < minGitLabSegments:
		return Location{}, errRepositoryURL
	}
	for _, seg := range segments {
		if !segmentPattern.MatchString(seg) || seg == "." || seg == ".." {
			return Location{}, errRepositoryURL
		}
	}
	host := strings.ToLower(u.Host)
	return Location{Host: host, FullName: path, WebURL: u.Scheme + "://" + host + pathSeparator + path}, nil
}
