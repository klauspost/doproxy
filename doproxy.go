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
	s, err := server.NewServer(*configfile)
	if err != nil {
		log.Fatal("Error loading server configuration:", err)
	}
	s.Run()
}
