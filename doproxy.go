package main

import (
	"flag"
	"fmt"
	"github.com/klauspost/doproxy/server"
	"github.com/klauspost/shutdown"
	"log"
	"os"
	"syscall"
	"time"
)

var configfile = flag.String("config", "doproxy.toml", "Use this config file")

func main() {
	//
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-options] [command]\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println("Commands: (if none is given the doproxy server is started)")
		fmt.Println(`  create "optinal-name"`)
		fmt.Println(`      Create a new backend and add it as a backend to the configuration.`)
		fmt.Println(`      If no name is given a name is generated.`)
		fmt.Println(`  delete id`)
		fmt.Println(`      Delete a backend with the given id.`)
	}
	flag.Parse()
	shutdown.Logger = log.New(os.Stdout, "", log.LstdFlags)
	shutdown.OnSignal(0, os.Interrupt, syscall.SIGTERM)
	shutdown.SetTimeout(time.Second)
	args := flag.Args()
	if len(args) == 0 {
		s, err := server.NewServer(*configfile)
		if err != nil {
			log.Fatal("Error loading server configuration:", err)
		}
		s.Run()
		return
	}
	cmd := args[0]
	conf, err := server.ReadConfigFile(*configfile)
	if err != nil {
		log.Fatal("Error loading server configuration:", err)
	}
	// We do not want health checks to be running.
	conf.Backend.DisableHealth = true
	switch cmd {
	case "create":
		name := ""
		if len(args) >= 2 {
			name = args[1]
		}
		drop, err := server.CreateDroplet(*conf, name)
		if err != nil {
			log.Fatal("Error creating droplet:", err)
		}
		log.Println("Adding droplet to inventory")
		inv, err := server.ReadInventory(conf.InventoryFile, conf.Backend)
		if err != nil {
			log.Fatal("Error loading inventory:", err)
		}
		be := server.NewDropletBackend(*drop, conf.Backend)
		err = inv.AddBackend(be)
		if err != nil {
			log.Fatal("Error adding droplet to inventory:", err)
		}
		err = inv.SaveDroplets(conf.InventoryFile)
		if err != nil {
			log.Fatal("Error saving new inventory:", err)
		}
		log.Println("New inventory saved.")
	case "delete":
		name := ""
		if len(args) >= 2 {
			name = args[1]
		}
		_ = name
	case "help":
		flag.Usage()
	default:
		flag.Usage()
		log.Println("Unknown command:", cmd)
		os.Exit(1)
	}
}
