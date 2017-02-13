package provider

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containous/traefik/log"
	"github.com/containous/traefik/safe"
	"github.com/containous/traefik/types"
	"gopkg.in/fsnotify.v1"
	"io/ioutil"
	"fmt"
	"path"
	"time"
)

var _ Provider = (*File)(nil)

// File holds configurations of the File provider.
type File struct {
	BaseProvider `mapstructure:",squash"`
	Directory string `description:"Read config from files on this directory"`
}

// Provide allows the provider to provide configurations to traefik
// using the given configuration channel.
func (provider *File) Provide(configurationChan chan<- types.ConfigMessage, pool *safe.Pool, constraints types.Constraints) error {
	if provider.Directory != "" {
		provider.handleDirectory(configurationChan, pool, constraints)
	} else {
		provider.handleSingleFile(configurationChan, pool, constraints)
	}

	return nil
}

func (provider *File) handleSingleFile(configurationChan chan<- types.ConfigMessage, pool *safe.Pool, constraints types.Constraints) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("Error creating file watcher", err)
		return err
	}

	file, err := os.Open(provider.Filename)
	if err != nil {
		log.Error("Error opening file", err)
		return err
	}
	defer file.Close()

	if provider.Watch {
		// Process events
		pool.Go(func(stop chan bool) {
			defer watcher.Close()
			for {
				select {
				case <-stop:
					return
				case event := <-watcher.Events:
					if strings.Contains(event.Name, file.Name()) {
						log.Debug("File event:", event)
						configuration := provider.loadFileConfig(file.Name())
						if configuration != nil {
							configurationChan <- types.ConfigMessage{
								ProviderName:  "file",
								Configuration: configuration,
							}
						}
					}
				case err := <-watcher.Errors:
					log.Error("Watcher event error", err)
				}
			}
		})
		err = watcher.Add(filepath.Dir(file.Name()))
		if err != nil {
			log.Error("Error adding file watcher", err)
			return err
		}
	}

	configuration := provider.loadFileConfig(file.Name())
	configurationChan <- types.ConfigMessage{
		ProviderName:  "file",
		Configuration: configuration,
	}
	return nil
}

func debounce(interval time.Duration, watcher *fsnotify.Watcher, stop chan bool, f func(arg fsnotify.Event)) {
	var (
		event fsnotify.Event
		eventptr *fsnotify.Event
	)

	for {
		select {
		case event = <-watcher.Events:
			eventptr = &event
		case <-time.After(interval):
			if eventptr != nil {
				f(event)
				eventptr = nil
			}
		case <- stop:
			return
		case err := <-watcher.Errors:
			log.Error("Watcher event error", err)
		}
	}
}

func (provider *File) handleDirectory(configurationChan chan<- types.ConfigMessage, pool *safe.Pool, constraints types.Constraints) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("Error creating directory watcher", err)
		return err
	}

	if provider.Watch {
		// Process events
		pool.Go(func(stop chan bool) {
			defer watcher.Close()
			debounce(5*time.Second, watcher, stop, func(event fsnotify.Event) {
				provider.loadFileConfigFromDir(configurationChan)
			})
		})

		err = watcher.Add(provider.Directory)
		if err != nil {
			log.Error("Error adding directory watcher", err)
			return err
		}
	}

	provider.loadFileConfigFromDir(configurationChan)

	return nil
}

func (provider *File) loadFileConfigFromDir(configurationChan chan<- types.ConfigMessage) error {
	files, err := ioutil.ReadDir(provider.Directory)

	if err != nil {
		log.Error(fmt.Sprintf("Unable to read Directory %s", provider.Directory), err)
		return err
	}

	configuration := new(types.Configuration)
	configuration.Frontends = make(map[string]*types.Frontend)
	configuration.Backends = make(map[string]*types.Backend)

	for _, file := range files {
		if(! strings.HasSuffix(file.Name(), ".toml")) {
			continue
		}

		c := provider.loadFileConfig(path.Join(provider.Directory, file.Name()))

		for k, v := range c.Backends {
			configuration.Backends[k] = v
		}

		for k, v := range c.Frontends {
			configuration.Frontends[k] = v
		}
	}

	configurationChan <- types.ConfigMessage{
		ProviderName:  "file",
		Configuration: configuration,
	}

	return nil

}

func (provider *File) loadFileConfig(filename string) *types.Configuration {
	configuration := new(types.Configuration)
	if _, err := toml.DecodeFile(filename, configuration); err != nil {
		log.Error("Error reading file:", err)
		return nil
	}
	return configuration
}
