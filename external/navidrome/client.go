package navidrome

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type NavidromeClient struct {
	URL      string
	User     string
	Password string
}

type NavidromePlaylist struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"songCount"`
}

type NavidromeTrack struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

func (c *NavidromeClient) buildURL(endpoint string, params url.Values) string {
	salt := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := md5.Sum([]byte(c.Password + salt))
	token := hex.EncodeToString(hash[:])

	if params == nil {
		params = url.Values{}
	}
	params.Set("u", c.User)
	params.Set("t", token)
	params.Set("s", salt)
	params.Set("v", "1.0.0")
	params.Set("c", "cliamp")
	params.Set("f", "json")

	return fmt.Sprintf("%s/rest/%s?%s", c.URL, endpoint, params.Encode())
}

func (c *NavidromeClient) GetPlaylists() ([]NavidromePlaylist, error) {
	log.Printf("Navidrome: Fetching playlists (user: %s)", c.User)

	resp, err := http.Get(c.buildURL("getPlaylists", nil))
	if err != nil {
		log.Printf("Navidrome Error (getPlaylists HTTP Get): %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Navidrome: getPlaylists returned status %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Navidrome Error: non-200 status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("navidrome API error: %s", resp.Status)
	}

	var result struct {
		SubsonicResponse struct {
			Playlists struct {
				Playlist []NavidromePlaylist `json:"playlist"`
			} `json:"playlists"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Navidrome Error (getPlaylists JSON Decode): %v", err)
		return nil, err
	}

	log.Printf("Navidrome: successfully parsed %d playlists", len(result.SubsonicResponse.Playlists.Playlist))
	return result.SubsonicResponse.Playlists.Playlist, nil
}

// GetPlaylistTracks fetches the tracks for a given playlist ID.
func (c *NavidromeClient) GetPlaylistTracks(id string) ([]NavidromeTrack, error) {
	log.Printf("Navidrome: Fetching tracks for playlist ID: %s", id)

	resp, err := http.Get(c.buildURL("getPlaylist", url.Values{"id": {id}}))
	if err != nil {
		log.Printf("Navidrome Error (getPlaylist HTTP Get): %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Navidrome: getPlaylist returned status %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Navidrome Error: non-200 status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("navidrome API error: %s", resp.Status)
	}

	var result struct {
		SubsonicResponse struct {
			Playlist struct {
				Entry []NavidromeTrack `json:"entry"`
			} `json:"playlist"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Navidrome Error (getPlaylist JSON Decode): %v", err)
		return nil, err
	}

	log.Printf("Navidrome: successfully parsed %d tracks", len(result.SubsonicResponse.Playlist.Entry))
	return result.SubsonicResponse.Playlist.Entry, nil
}

// StreamURL generates the authenticated streaming URL for a track ID.
func (c *NavidromeClient) StreamURL(id string) string {
	log.Printf("Navidrome: Generating stream URL for track ID: %s", id)
	return c.buildURL("stream", url.Values{"id": {id}, "format": {"mp3"}})
}
