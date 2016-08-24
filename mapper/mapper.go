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
	Clip []ClipResponse `json:"clips"`
}

type ClipResponse struct {
	Type string `json:"type"`
	Path string `json:"path"`
}


// NewMapper creates a new mapper
func NewMapper(config *Config, statter statsd.Statter) *Mapper {
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

// MultiURLMapper maps a single item in a multi-url set
func (m *Mapper) MultiURLMapper(w http.ResponseWriter, r* http.Request) {
	w.Header().Set("Server", serverName)

	if r.Method != "HEAD" && r.Method != "GET" {
		w.WriteHeader(405)
		return
	}

	// Example incoming path:
	//    /multimap/evs/w5e0x2kuzklnklp_2414279.mp4

	logging.Debugf("Mapper: request for URL: %s", r.URL.String())

	parts := strings.SplitN(r.URL.String(), "/", 4)

	if len(parts) != 4 {
		w.WriteHeader(400)
		return
	}

	parts = strings.SplitN(parts[3], "_", 2)
	if len(parts) != 2 {
		w.WriteHeader(400)
		return
	}

	mediaID := parts[0]
	namePart := parts[1]
	encodeID := strings.TrimSuffix(namePart, ".mp4")

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
		Sequences: []SequenceItemResponse{
			SequenceItemResponse{
				Clip: []ClipResponse{
					ClipResponse{
						Type: "source",
						Path: fmt.Sprintf("http://127.0.0.1:8080/media/%s/%s", mediaID, namePart),
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
