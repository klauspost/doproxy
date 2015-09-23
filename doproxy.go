package main

import (
	"github.com/klauspost/doproxy/server"
	"github.com/klauspost/shutdown"
	"os"
	"syscall"
	"time"
	"log"
)

func main() {
	shutdown.Logger = log.New(os.Stdout, "", log.LstdFlags)
	shutdown.OnSignal(0, os.Interrupt, syscall.SIGTERM)
	shutdown.SetTimeout(time.Second)
	s, err := server.NewServer("doproxy.toml")
	if err != nil {
		log.Fatal("Error loading server configuration:", err)
	}
	s.Run()
}
