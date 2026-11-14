package main

import (
	"log"
	"os"

	"github.com/sspencer/atc/commands"
)

func main() {
	log.SetFlags(0)
	if err := commands.Execute(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
