package mapper

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/crunchyroll/evs-common/logging"
	"github.com/crunchyroll/evs-playback-api/cache"
	"github.com/crunchyroll/evs-playback-api/objects"
	"github.com/crunchyroll/evs-playback-api/schema"
)

// serverName has a header value to add
const serverName = "Ellation Video Playback Mapping Service"

// Config holds our config information
type Config struct {
	Store         objects.Config `yaml:"manifests" optional:"true"`
	Cache         cache.Config   `yaml:"config" optional:"true"`
	CaptionServer string         `yaml:"caption_server" optional:"true"`
}

// Mapper holds instance state
type Mapper struct {
	config  *Config
	store   objects.ObjectStore
	statter statsd.Statter
	cache   *cache.Cache
}

// CachedResponse holds cached responses for a given video ID
type CachedResponses struct {
	videoID   cache.Key
	lock      sync.Mutex
	manifest  *schema.Manifest
	etag      string
	responses map[string]*[]byte
}

// Response contains a mapping response
type Response struct {
	Sequences []SequenceItemResponse `json:"sequences"`
}

type SequenceItemResponse struct {
	Language string         `json:"language,omitempty"`
	Label    string         `json:"label,omitempty"`
	Clip     []ClipResponse `json:"clips"`
}

type ClipResponse struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// NewMapper creates a new mapper
func NewMapper(config *Config, statter statsd.Statter) *Mapper {
	logging.Infof("Using manifest object store %q", config.Store.StoreName)

	store, err := objects.NewObjectStore(&config.Store)
	if err != nil {
		logging.Panicf("Unable to construct object store %q: %v", config.Store.StoreName, err)
	}

	mapper := &Mapper{
		config:  config,
		store:   store,
		statter: statter,
	}

	mapper.cache = cache.NewCache(&config.Cache, mapper, statter)
	return mapper
}

// MapManifest maps the manifest requested
func (m *Mapper) MapManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", serverName)

	if r.Method != "HEAD" && r.Method != "GET" {
		w.WriteHeader(405)
		return
	}

	// Example incoming path:
	//    single   - /map/evs/assets/w5e0x2kuzklnklp/2414279.mp4
	//    adaptive - /map/evs/assets/w5e0x2kuzklnklp

	logging.Debugf("MapManifest: request for URL: %s", r.URL.String())

	parts := strings.Split(r.URL.String(), "/")

	if len(parts) == 5 {
		m.MapManifestAdaptive(w, r)
		return
	}

	if len(parts) != 6 {
		w.WriteHeader(400)
		return
	}

	mediaID := parts[4]
	namePart := parts[5]
	encodeID := strings.TrimSuffix(namePart, ".mp4")

	var response = Response{
		Sequences: []SequenceItemResponse{
			SequenceItemResponse{
				Clip: []ClipResponse{
					ClipResponse{
						Type: "source",
						Path: fmt.Sprintf("/media/%s/%s", mediaID, namePart),
					},
				},
			},
		},
	}

	body, err := json.Marshal(&response)
	if err != nil {
		logging.Errorf("Failed to marshal response to MediaID %q, EncodeID %q: %v", mediaID, encodeID, err)
		w.WriteHeader(503)
		return
	}

	w.WriteHeader(200)

	_, _ = w.Write(body)

	logging.Debugf(string(body))

}

// Map alang/hlang to language, label
var languageMap = map[string]string{
	"koKR":    "kor",
	"jpJP":    "jpn",
	"jaJP":    "jpn",
	"enUS":    "eng",
	"en-US":   "eng",
	"en":      "eng",
	"esES":    "spa",
	"es":      "spa",
	"esLA":    "spa",
	"ptPT":    "por",
	"ptBR":    "por",
	"pt":      "por",
	"frFR":    "fra",
	"deDE":    "deu",
	"itIT":    "ita",
	"nlNL":    "dut",
	"zh-Hans": "chi",
	"zh-Hant": "chi",
	"zhHK":    "chi",
	"arME":    "ara",
}

var labelMap = map[string]string{
	"koKR":    "Korean",
	"jpJP":    "Japanese",
	"jaJP":    "Japanese",
	"enUS":    "English",
	"en-US":   "English",
	"en":      "English",
	"esES":    "Spanish",
	"es":      "Spanish",
	"esLA":    "Spanish Latin America",
	"ptPT":    "Portuguese",
	"ptBR":    "Portuguese",
	"pt":      "Portuguese",
	"frFR":    "French",
	"deDE":    "German",
	"itIT":    "Italian",
	"nlNL":    "Dutch",
	"zh-Hans": "Chinese",
	"zh-Hant": "Chinese",
	"zhHK":    "Chinese",
	"arME":    "Arabic",
}

func getLanguage(lang string) (lang3, label string) {
	if strings.Contains(lang, ".") {
		l := strings.SplitN(lang, ".", 2)
		lang = l[0]
	}

	var ok bool
	if lang3, ok = languageMap[lang]; !ok {
		logging.Infof("No language code for %q", lang)
		lang3 = ""
		label = ""
	}

	if label, ok = labelMap[lang]; !ok {
		logging.Infof("No language label for %q", lang)
		lang3 = ""
		label = ""
	}
	return
}

// MultiMap maps a single item in a multi-url set to a sequence
func (m *Mapper) MapManifestAdaptive(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.String(), "/", 5)

	if len(parts) != 5 {
		w.WriteHeader(400)
		return
	}

	mediaID := strings.TrimSuffix(parts[4], ".mp4")

	cached, err := m.cache.Get(cache.Key(mediaID))
	if err != nil {
		logging.Errorf("Unable to obtain manifest for media %q: %v", mediaID, err)
		w.WriteHeader(err.(*cache.CacheError).StatusCode())
		return
	}

	// Use cached response if available
	c := cached.(*CachedResponses)
	if response, ok := c.responses[r.URL.Path]; ok {
		w.WriteHeader(200)
		_, _ = w.Write(*response)
		m.cache.TrackHit()
		return
	}

	m.cache.TrackMiss()

	logging.Debugf("Cache miss - populating")

	man := c.manifest

	var alang, label string
	if man.Alang != "" {
		alang, label = getLanguage(man.Alang)
	}

	var response = Response{
		Sequences: []SequenceItemResponse{},
	}

	for i := range man.Encodes {
		encode := &man.Encodes[i]

		eAlang := alang
		eLabel := label
		if encode.Alang != "" {
			eAlang, eLabel = getLanguage(encode.Alang)
		}

		if encode.Hlang == "" && encode.Quality != "trailer" {
			response.Sequences = append(response.Sequences, SequenceItemResponse{
				Language: eAlang,
				Label:    eLabel,
				Clip: []ClipResponse{
					ClipResponse{
						Type: "source",
						Path: fmt.Sprintf("/media/%s/%s", mediaID, encode.File),
					},
				},
			})
		}
	}

	for i := range man.Subtitles {
		subtitle := &man.Subtitles[i]
		if strings.HasSuffix(subtitle.File, ".txt") {
			lang, label := getLanguage(subtitle.Language)
			dynamicAsset := fmt.Sprintf("%s.vtt", strings.TrimSuffix(subtitle.File, ".txt"))
			response.Sequences = append(response.Sequences, SequenceItemResponse{
				Language: lang,
				Label:    label,
				Clip: []ClipResponse{
					ClipResponse{
						Type: "source",
						Path: fmt.Sprintf("/caption/%s/%s", mediaID, dynamicAsset),
					},
				},
			})
		}
	}

	body, err := json.Marshal(&response)
	if err != nil {
		logging.Errorf("Failed to marshal adaptive response to MediaID %q: %v", mediaID, err)
		w.WriteHeader(503)
		return
	}

	w.WriteHeader(200)

	_, _ = w.Write(body)

	c.responses[r.URL.Path] = &body

	logging.Debugf(string(body))
}

// Populate is used by the cache to backfill - fetch manifest and return it as a Cacheable.  Errors
// are always cache.CacheError.
func (m Mapper) Populate(videoID cache.Key) (cache.Cacheable, error) {
	body, code, err := m.store.GetManifest(string(videoID))
	if err != nil || code < 200 || code >= 300 {
		if err == nil {
			err = fmt.Errorf("HTTP error fetching manifest")
		}
		return nil, cache.NewCacheError(err, code)
	}

	bodyBytes := []byte(body)
	hash := md5.Sum(bodyBytes)
	etag := fmt.Sprintf("%x", hash)

	response := &CachedResponses{
		videoID:   videoID,
		etag:      etag,
		responses: make(map[string]*[]byte),
	}

	var m3 schema.Manifest
	if err := json.Unmarshal(bodyBytes, &m3); err != nil {
		logging.Errorf("unable to unmarshal video manifest %s: %v", videoID, err)
		return nil, cache.NewCacheError(fmt.Errorf("Unable to unmarshal manifest for %s: %v", videoID, err), 0)
	}

	response.manifest = &m3
	return response, nil
}

// ETag is used by cache to obtain ETag for manifest
func (r CachedResponses) ETag() string {
	return r.etag
}

// Evicted is called when a cachedManifest is thrown out of the cache.
// Called with the entire cache and/or LRU list locked down so don't do much here.
func (r CachedResponses) Evicted() {
	return
}
