package config

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/reporter"
	"reflect"
	"testing"
)

func TestConfig_LoadConfig(t *testing.T) {
	_, err := LoadConfiguration("../.hunter.conf.example")

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
		Smtp:    nil,
		Webhook: nil,
	}

	paths := conf.GetPaths()
	want := []string{"./", "/path/to/scan", "/second/path"}

	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("%v does not match expected %v", paths, want)
	}
}

func TestConfig_GetReporters(t *testing.T) {
	conf := Config{
		Presets: nil,
		Smtp:    nil,
		Webhook: nil,
	}

	actualRouter := conf.GetReporters()

	expectedReporter := reporter.ChainReporter{}
	expectedReporter.Add(reporter.StdOut{})

	if !reflect.DeepEqual(actualRouter, expectedReporter) {
		t.Errorf("returned router not expected: %#v", actualRouter)
	}
}
