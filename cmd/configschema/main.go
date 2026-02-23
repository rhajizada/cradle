package main

import (
	"encoding/json"
	"log"
	"os"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/rhajizada/cradle/internal/config"
)

func main() {
	opts := &jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[config.DeviceCount](): deviceCountSchema(),
		},
	}

	s, err := jsonschema.For[config.Config](opts)
	if err != nil {
		log.Fatal(err)
	}

	s.Schema = "http://json-schema.org/draft-07/schema#"

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if encodeErr := enc.Encode(s); encodeErr != nil {
		log.Fatal(encodeErr)
	}
}

func deviceCountSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "integer"},
			{Type: "string", Enum: []any{"all"}},
		},
	}
}
