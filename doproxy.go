package main

import (
	"flag"
	"github.com/klauspost/doproxy/server"
	"github.com/klauspost/shutdown"
	"log"
	"os"
	"syscall"
	"time"
)

var configfile = flag.String("config", "doproxy.toml", "Use this config file")

func main() {
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
	}
}
