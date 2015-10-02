package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"strconv"

	"github.com/klauspost/doproxy/server"
	"github.com/klauspost/shutdown"
)

var configfile = flag.String("config", "doproxy.toml", "Use this config file")

func main() {
	//
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-options] [command]\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println("Commands: (if none is given the doproxy server is started)")
		fmt.Println(`  add <id>`)
		fmt.Println(`      Add a running droplet to your inventory.`)
		fmt.Println(`  create "optinal-name"`)
		fmt.Println(`      Create a new backend and add it as a backend to the configuration.`)
		fmt.Println(`      If no name is given a name is generated.`)
		fmt.Println(`  delete <id>`)
		fmt.Println(`      Delete a backend with the given id.`)
		fmt.Println(`  list`)
		fmt.Println(`      List all currently running droplets.`)
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
	case "list":
		drops, err := server.ListDroplets(*conf)
		if err != nil {
			log.Fatal("Error listing droplets:", err)
		}
		fmt.Printf("%d Currently Running:\n", len(drops.Droplets))
		for _, drop := range drops.Droplets {
			fmt.Println("[[droplet]]\n" + drop.String())
		}
	case "add":
		if len(args) < 2 {
			log.Fatal("add: No ID supplied")
		}
		sid := args[1]
		id, err := strconv.Atoi(sid)
		if err != nil {
			log.Fatalf("%q is not a valid ID. It must be a number ", sid)
		}

		inv, err := server.ReadInventory(conf.InventoryFile, conf.Backend)
		if err != nil {
			log.Fatal("Error loading inventory:", err)
		}
		_, ok := inv.BackendID(sid)
		if ok {
			log.Fatalf("Droplet with id %q already exists in inventory", sid)
		}
		drops, err := server.ListDroplets(*conf)
		if err != nil {
			log.Fatal("Error listing droplets:", err)
		}
		drop, ok := drops.DropletID(id)
		if !ok {
			log.Fatal("Unable to locate a running droplet with ID ", sid)
		}
		be, err := drop.ToBackend(conf.Backend)
		if err != nil {
			log.Fatal("Error listing droplets:", err)
		}
		err = inv.AddBackend(be)
		if err != nil {
			log.Fatal("Error adding backend:", err)
		}
		err = inv.SaveDroplets(conf.InventoryFile)
		if err != nil {
			log.Fatal("Error saving inventory:", err)
		}
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
