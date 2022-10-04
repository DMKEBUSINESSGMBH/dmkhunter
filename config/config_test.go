package config

import (
	"reflect"
	"testing"
)

func TestConfig_LoadConfig(t *testing.T) {
	_, err := LoadConfiguration("../.hunter.conf")

	if err != nil {
		t.Fatalf("error loading config file: %q", err)
	}
}

func TestConfig_GetPaths(t *testing.T) {
	conf := Config{
		Presets: []Preset{
			{
				Paths:    []string{"./"},
				Clamav:   nil,
				Database: nil,
			},
			{
				Paths:    []string{"/path/to/scan", "/second/path"},
				Clamav:   nil,
				Database: nil,
			},
		},
		smtp:    nil,
		webhook: nil,
	}

	paths := conf.GetPaths()
	want := []string{"./", "/path/to/scan", "/second/path"}

	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("%v does not match expected %v", paths, want)
	}
}
