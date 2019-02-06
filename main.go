package main

import (
	"os"
	"fmt"
	"encoding/json"
)

const ConfigFilename string = "config.json"


func main() {
	var config Config
	file, err := os.Open(ConfigFilename)
	if err != nil {
		fmt.Printf("Failed to open %s\n", ConfigFilename)
		return
	}
	decoder := json.NewDecoder(file) 
	err = decoder.Decode(&config) 
	if err != nil {
		fmt.Printf("Failed decode config\n")
		return
	}

	var s Server
	s.initElasticClient(config.ElasticUrl)
	s.setRoutes()
	s.listen(config.Port)
}