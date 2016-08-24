package mapper

import (
	"net/http"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/crunchyroll/evs-playback-api/objects"
	"go.codemobs.com/vps/common/logging"
)

// Server header value to add
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

// ManifestHandler maps a manifest.json object into a Kaltura VOD format manifest.
func (m *Mapper) ManifestHandler(w http.ResponseWriter, r* http.Request) {
	if r.Method != "HEAD" && r.Method != "GET" {
		w.WriteHeader(405)
		return
	}

	logging.Debugf("Request for URL: %s", r.URL.String())
	w.WriteHeader(400)
}
