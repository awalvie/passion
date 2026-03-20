package web

import (
	"net/url"
	"strings"
)

// youtubeEmbedURL returns a https://www.youtube.com/embed/... URL for iframes when raw is a
// recognized YouTube watch, shorts, live, youtu.be, or embed link. Otherwise returns "" so
// callers can fall back to a plain HTML5 <video src="..."> for direct files (.mp4, etc.).
func youtubeEmbedURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}

	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "youtube.com" && host != "youtube-nocookie.com" && host != "m.youtube.com" && host != "youtu.be" {
		return ""
	}

	// youtu.be/VIDEO_ID
	if host == "youtu.be" {
		id := strings.Trim(u.Path, "/")
		id = strings.Split(id, "/")[0]
		if id == "" || strings.Contains(id, ".") {
			return ""
		}
		return "https://www.youtube.com/embed/" + id + "?rel=0"
	}

	path := u.Path

	// /embed/VIDEO_ID (already embeddable)
	if strings.HasPrefix(path, "/embed/") {
		id := strings.TrimPrefix(path, "/embed/")
		id = strings.Split(id, "/")[0]
		if id == "" {
			return ""
		}
		return "https://www.youtube.com/embed/" + id + "?rel=0"
	}

	// /watch?v=VIDEO_ID
	if path == "/watch" || strings.HasPrefix(path, "/watch/") {
		v := u.Query().Get("v")
		if v != "" {
			return "https://www.youtube.com/embed/" + v + "?rel=0"
		}
	}

	// /shorts/VIDEO_ID
	if strings.HasPrefix(path, "/shorts/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "shorts" && parts[1] != "" {
			return "https://www.youtube.com/embed/" + parts[1] + "?rel=0"
		}
	}

	// /live/VIDEO_ID
	if strings.HasPrefix(path, "/live/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "live" && parts[1] != "" {
			return "https://www.youtube.com/embed/" + parts[1] + "?rel=0"
		}
	}

	return ""
}
