package main

import (
	"log"

	"github.com/shirou/gopsutil/load"
)

func main() {
	avg, err := load.Avg()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", avg.String())
}
