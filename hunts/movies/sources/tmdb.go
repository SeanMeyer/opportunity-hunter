package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// TMDB fetches movies from The Movie Database API.
type TMDB struct {
	apiKey     string
	httpClient *http.Client
}

func NewTMDB(apiKey string, httpClient *http.Client) *TMDB {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &TMDB{apiKey: apiKey, httpClient: httpClient}
}

func (t *TMDB) Name() string { return "tmdb" }

func (t *TMDB) Scan(ctx context.Context, _ core.ScanRegion) ([]core.RawItem, error) {
	var allItems []core.RawItem

	// Fetch now playing (theatrical).
	playing, err := t.fetchList(ctx, "/3/movie/now_playing", "theatrical")
	if err != nil {
		return nil, fmt.Errorf("now_playing: %w", err)
	}
	allItems = append(allItems, playing...)

	// Fetch upcoming (theatrical).
	upcoming, err := t.fetchList(ctx, "/3/movie/upcoming", "theatrical")
	if err != nil {
		return nil, fmt.Errorf("upcoming: %w", err)
	}
	allItems = append(allItems, upcoming...)

	return allItems, nil
}

func (t *TMDB) fetchList(ctx context.Context, path, releaseType string) ([]core.RawItem, error) {
	url := fmt.Sprintf("https://api.themoviedb.org%s?api_key=%s&language=en-US&page=1", path, t.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb %d: %s", resp.StatusCode, string(body))
	}

	var result tmdbListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var items []core.RawItem
	for _, m := range result.Results {
		item := tmdbMovieToRawItem(m, releaseType)
		if item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

func tmdbMovieToRawItem(m tmdbMovie, releaseType string) *core.RawItem {
	if m.Title == "" || m.ReleaseDate == "" {
		return nil
	}

	releaseTime, _ := time.Parse("2006-01-02", m.ReleaseDate)

	genres := genreNames(m.GenreIDs)

	// Encode attrs as raw JSON to avoid import cycle with parent package.
	attrs, _ := json.Marshal(map[string]any{
		"tmdb_rating":  m.VoteAverage,
		"release_type": releaseType,
		"genre":        genres,
	})

	item := &core.RawItem{
		SourceID:   fmt.Sprintf("tmdb-%d", m.ID),
		Source:     "tmdb",
		Title:      m.Title,
		Subtitle:   strings.Join(genres, ", "),
		StartTime:  releaseTime.Format(time.RFC3339),
		Attributes: attrs,
	}

	movieJSON, _ := json.Marshal(m)
	item.RawJSON = string(movieJSON)

	return item
}

// Simplified genre mapping — full map would come from TMDB /genre/movie/list.
func genreNames(ids []int) []string {
	genreMap := map[int]string{
		28: "Action", 12: "Adventure", 16: "Animation", 35: "Comedy",
		80: "Crime", 99: "Documentary", 18: "Drama", 10751: "Family",
		14: "Fantasy", 36: "History", 27: "Horror", 10402: "Music",
		9648: "Mystery", 10749: "Romance", 878: "Sci-Fi", 53: "Thriller",
		10752: "War", 37: "Western",
	}
	var names []string
	for _, id := range ids {
		if name, ok := genreMap[id]; ok {
			names = append(names, name)
		}
	}
	return names
}

type tmdbListResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	GenreIDs    []int   `json:"genre_ids"`
}
