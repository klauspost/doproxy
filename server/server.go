package server

import (
	"github.com/klauspost/shutdown"
	"gopkg.in/fsnotify.v1"
	"log"
	"net/http"
	"sync"
)

// Server contains the main server configuration
// and server-wide information.
// Since there is no global data, it is possible
// to run multiple servers at once with different
// configurations.
type Server struct {
	Config  Config
	mu      sync.RWMutex
	handler *ReverseProxy
	exitMonInv chan chan struct{}  // Channel to indicate that inventory monitoring must stop.
}

// NewServer will read the supplied config file,
// and return a new server.
// A file watcher will be set up to monitor the
// configuration file and reload settings if changes
// are detected.
func NewServer(config string) (*Server, error) {
	s := &Server{handler: NewReverseProxy()}
	err := s.ReadConfig(config, true)
	if err != nil {
		return nil, err
	}

	// Add config file watcher/reloader.
	if s.Config.WatchConfig {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		err = watcher.Add(config)
		if err != nil {
			return nil, err
		}
		log.Println("Watching", config)
		// We want the watcher to exit in the first stage.
		go func() {
			// Get a first stage shutdown notification
			exit := shutdown.First()
			for {
				select {
				// Event on config file.
				case event := <-watcher.Events:
					switch event.Op {
					// Editor may do rename -> write -> delete, so we should not follow
					// the old file
					case fsnotify.Rename:
						watcher.Remove(event.Name)
						watcher.Add(config)
					case fsnotify.Remove:
						continue
					}
					log.Println("Reloading configuration")
					err := s.ReadConfig(event.Name, false)
					if err != nil {
						log.Println("Error reloading configuration:", err)
						log.Println("Configuration NOT applied")
					} else {
						log.Println("Configuration applied")
					}

					// Server is shutting down
				case n := <-exit:
					watcher.Remove(config)
					close(n)
					return
				}
			}
		}()
	}
	return s, nil
}

// MonitorInventory will monitor the inventory file
// and reload the inventory if changes are detected.
// The monitor can be shut down by sending a channel on
// (Server).exitMonInv. The monitor will exit and close
// the supplied channel.
func (s *Server) MonitorInventory() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	file := s.Config.InventoryFile
	err = watcher.Add(file)
	if err != nil {
		return err
	}

	// Create channel to stop monitoring
	stop := make(chan chan struct{})
	s.exitMonInv = stop

	log.Println("Watching", file)
	// We want the watcher to exit in the first stage.
	go func() {
		// Get a first stage shutdown notification
		exit := shutdown.First()
		for {
			select {
			// Event on config file.
			case event := <-watcher.Events:
				switch event.Op {
				// Editor may do rename -> write -> delete, so we should not follow
				// the old file
				case fsnotify.Rename:
					watcher.Remove(event.Name)
					watcher.Add(file)
				case fsnotify.Remove:
					continue
				}
				log.Println("Reloading inventory")
				s.mu.RLock()
				bec := s.Config.Backend
				s.mu.RUnlock()

				inv, err := ReadInventory(event.Name, bec)
				if err != nil {
					log.Println("Error reloading inventory:", err)
					log.Println("New inventory NOT applied")
					continue
				}

				// Update the load balancer
				s.mu.RLock()
				lb, err := NewLoadBalancer(s.Config.LoadBalancing, inv)
				if err != nil {
					log.Println(err)
					log.Println("New inventory NOT applied")
					s.mu.RUnlock()
					continue
				}
				s.handler.SetBackends(lb)
				s.mu.RUnlock()

				log.Println("New inventory applied")
			// Server is shutting down
			case n := <-exit:
				log.Println("Monitor exiting")
				watcher.Remove(file)
				close(n)
				return
				// Monitor must stop
			case n := <-stop:
				exit.Cancel()
				close(n)
				return
			}
		}
	}()
	return nil
}

// Run the server.
func (s *Server) Run() {
	// Read inventory
	inv, err := ReadInventory(s.Config.InventoryFile, s.Config.Backend)
	if err != nil {
		log.Fatal(err)
	}

	//err = inv.Save("inventory-saved.toml")
	//if err != nil {
	//	log.Fatal(err)
	//}

	// Create a load balancer and apply it.
	lb, err := NewLoadBalancer(s.Config.LoadBalancing, inv)
	if err != nil {
		log.Fatal(err)
	}
	s.handler = NewReverseProxyConfig(s.Config, lb)

	// Start monitoring inventory.
	s.MonitorInventory()

	mux := http.NewServeMux()
	mux.Handle("/", s.handler)

	srv := &http.Server{Handler: mux, Addr: s.Config.Bind}
	if s.Config.Https {
		err := srv.ListenAndServeTLS(s.Config.CertFile, s.Config.KeyFile)
		if err != nil {
			log.Fatalf("Starting HTTPS frontend failed: %v", err)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Starting frontend failed: %v", err)
		}
	}
}
