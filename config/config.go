package config

import (
    "github.com/hashicorp/hcl"
    "fmt"
    "io/ioutil"
)

type Comparator string
const (
    GreaterThan          Comparator = ">"
    GreaterThanOrEqualTo Comparator = ">="
    LesserThanOrEqualTo  Comparator = "<="
    LesserThan           Comparator = "<"
)


type Check struct {
    Plugin string `hcl:"plugin"`
    PluginInstance string `hcl:"plugin_instance"`
    Type string `hcl:"type"`
    TypeInstance string `hcl:"type_instance"`
    Comparator Comparator `hcl:"comparator"`
    Warning string `hcl:"warning"`
    Critical string `hcl:"critical"`
}

type Config struct {
    Checks map[string]Check `hcl:"check"`
}

func Read() (*Config, error) {
    out, err := loadFileHcl("config.hcl")
    if err != nil {
        return nil, err
    }
    return out, nil
}

// loadFileHcl is a fileLoaderFunc that knows how to read HCL files.
func loadFileHcl(root string) (*Config, error) {
    // Read the HCL file and prepare for parsing
    d, err := ioutil.ReadFile(root)
    if err != nil {
        return nil, fmt.Errorf("Error reading %s: %s", root, err)
    }

    // Decode it
    var out Config
    if err = hcl.Decode(&out, string(d)); err != nil {
        return nil, fmt.Errorf("Error decoding %s: %s", root, err)
    }

    return &out, nil
}
