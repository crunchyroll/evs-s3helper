package mapper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/crunchyroll/evs-playback-api/objects"
	"github.com/crunchyroll/evs-playback-api/schema"
	"go.codemobs.com/vps/common/logging"
)

// serverName has a header value to add
const serverName = "Ellation Video Playback Mapping Service"

// Config holds our config information
type Config struct {
	Store objects.Config `yaml:"manifests" optional:"true"`
}

// Mapper holds instance state
type Mapper struct {
	config  *Config
	store   objects.ObjectStore
	statter statsd.Statter
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

	return &Mapper{
		config:  config,
		store:   store,
		statter: statter,
	}
}

// MapManifest maps the manifest requested
func (m *Mapper) MapManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", serverName)

	if r.Method != "HEAD" && r.Method != "GET" {
		w.WriteHeader(405)
		return
	}

	// Example incoming path:
	//    single   - /map/evs/w5e0x2kuzklnklp/2414279.mp4
	//    adaptive - /map/evs/w5e0x2kuzklnklp.mp4

	logging.Debugf("MapManifest: request for URL: %s", r.URL.String())

	parts := strings.Split(r.URL.String(), "/")

	if len(parts) == 4 {
		m.MapManifestAdaptive(w, r)
		return
	}

	if len(parts) != 5 {
		w.WriteHeader(400)
		return
	}

	mediaID := parts[3]
	namePart := parts[4]
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
}

var labelMap = map[string]string{
	"koKR":    "Korean",
	"jpJP":    "Japanese",
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
}

func getLanguage(lang string) (lang3, label string) {
	if strings.Contains(lang, ".") {
		l := strings.SplitN(lang, ".", 2)
		lang = l[0]
	}

	var ok bool
	if lang3, ok = languageMap[lang]; !ok {
		logging.Infof("No language code for %q", lang)
		lang3 = "ZZZ"
	}

	if label, ok = labelMap[lang]; !ok {
		logging.Infof("No language label for %q", lang)
		label = "Unknown"
	}
	return
}

// MultiMap maps a single item in a multi-url set to a sequence
func (m *Mapper) MapManifestAdaptive(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.String(), "/", 4)

	if len(parts) != 4 {
		w.WriteHeader(400)
		return
	}

	mediaID := strings.TrimSuffix(parts[3], ".mp4")

	manifest, statusCode, err := m.store.GetManifest(mediaID)
	if err != nil {
		logging.Errorf("Unable to obtain manifest for media %q: %v", mediaID, err)
		w.WriteHeader(statusCode)
		return
	}

	var man schema.Manifest
	err = json.Unmarshal([]byte(manifest), &man)
	if err != nil {
		logging.Errorf("Unable to parse manifest for media ID %q: %v", mediaID, err)
		w.WriteHeader(503)
		return
	}

	var response = Response{
		Sequences: []SequenceItemResponse{},
	}

	for i := range man.Encodes {
		encode := &man.Encodes[i]
		if encode.Hlang == "" && encode.Quality != "trailer" {
			response.Sequences = append(response.Sequences, SequenceItemResponse{
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
		if strings.HasSuffix(subtitle.File, ".vtt") {
			lang, label := getLanguage(subtitle.Language)
			response.Sequences = append(response.Sequences, SequenceItemResponse{
				Language: lang,
				Label:    label,
				Clip: []ClipResponse{
					ClipResponse{
						Type: "source",
						Path: fmt.Sprintf("/media/%s/%s", mediaID, subtitle.File),
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

	logging.Debugf(string(body))
}
