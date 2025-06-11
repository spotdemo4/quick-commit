package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spotdemo4/quick-commit/internal/tui"
)

type config struct {
	url     *url.URL
	model   string
	headers map[string]string
	options map[string]any
}

func getConfig() (c config, err error) {
	// Get .env file
	configDir, err := os.UserConfigDir()
	if err != nil {
		tui.PrintWarn("warning: could not get config dir: %v", err)
	} else {
		err := godotenv.Load(filepath.Join(configDir, "quick-commit.env"))
		if err != nil {
			tui.PrintWarn("warning: could not load %s", filepath.Join(configDir, "quick-commit.env"))
		}
	}

	// Get env vars
	urlStr := os.Getenv("QC_URL")
	if urlStr == "" {
		urlStr := "http://localhost:11434"
		tui.PrintWarn("warning: 'QC_URL' not set, defaulting to %s", urlStr)
	}
	c.url, err = url.Parse(urlStr)
	if err != nil {
		return c, fmt.Errorf("could not parse url: %w", err)
	}

	c.model = os.Getenv("QC_MODEL")
	if c.model == "" {
		return c, errors.New("'QC_MODEL' not set")
	}

	// Get headers and options
	c.headers = map[string]string{}
	c.options = map[string]any{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "QC_HEADER_") {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				continue
			}

			c.headers[strings.TrimPrefix(kv[0], "QC_HEADER_")] = kv[1]
			continue
		}

		if strings.HasPrefix(e, "QC_OPTION_") {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				continue
			}

			opt := strings.TrimPrefix(kv[0], "QC_OPTION_")

			if opt == "temperature" {
				float, err := strconv.ParseFloat(kv[1], 32)
				if err != nil {
					tui.PrintWarn("warning: invalid value for 'QC_OPTION_TEMPERATURE': %v", err)
					continue
				}

				c.options[opt] = float32(float)
				continue
			}

			c.options[strings.TrimPrefix(kv[0], "QC_OPTION_")] = kv[1]
			continue
		}
	}

	return c, nil
}
