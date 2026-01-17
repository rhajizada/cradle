package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/rhajizada/cradle/internal/config"
)

func main() {
	s, err := jsonschema.For[config.Config](nil)
	if err != nil {
		log.Fatal(err)
	}

	s.Schema = "http://json-schema.org/draft-07/schema#"

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		log.Fatal(err)
	}
}
