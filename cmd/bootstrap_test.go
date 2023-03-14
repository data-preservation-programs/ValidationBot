package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDefaultBoostrapConfig(t *testing.T) {
	assert := assert.New(t)

	path, err := os.Getwd()
	assert.NoError(err)

	dir, err := os.MkdirTemp(path, "config")
	assert.NoError(err)

	config_path := filepath.Join(dir, "config.yaml")
	defer os.RemoveAll(dir)

	config, err := setConfig(context.Background(), config_path)
	assert.NoError(err)

	defer viper.Reset()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// make sure we reach the root of the project
	absPath, err := filepath.Abs(filepath.Dir(fmt.Sprintf("%s/..", wd)))
	assert.NoError(err)

	filePath := filepath.Join(absPath, "validation_bot")

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		fmt.Printf("validation_bot binary exists: %s\n", filePath)
	} else {
		fmt.Printf("could not find validation_bot binary along path: %s\n", filePath)
	}

	fmt.Printf("comparing filepaths: %s and %s\n", config.Auditor.ClientRPC.ExecPath, absPath)
	assert.NotNil(config)
	assert.Equal(config.Auditor.ClientRPC.ExecPath, absPath)
}
