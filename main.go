package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/genuinetools/pkg/cli"
	"github.com/jessfraz/gmailfilters/version"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	gmailUser = "me"
)

/*
 * Flow
1. Read label map from gmail
2. parse local filter rules
3. for each rule, parse to gmail filter, create new labels if not in map
4. parse local label rules
5. *update* any labels that have local changes
6. merge label map + new rules with local change file, save to export file
*/

var (
	credsFile string

	tokenFile string

	filtersFile string

	labelsFile string

	api *gmail.Service

	debug bool

	export bool
)

func main() {
	// Create a new cli program.
	p := cli.NewProgram()
	p.Name = "gmailfilters"
	p.Description = "A tool to sync Gmail filters from a config file to your account"
	// Set the GitCommit and Version.
	p.GitCommit = version.GITCOMMIT
	p.Version = version.VERSION

	// Setup the global flags.
	p.FlagSet = flag.NewFlagSet("gmailfilters", flag.ExitOnError)
	p.FlagSet.BoolVar(&debug, "d", false, "enable debug logging")
	p.FlagSet.BoolVar(&debug, "debug", false, "enable debug logging")

	p.FlagSet.BoolVar(&export, "e", false, "export existing filters")
	p.FlagSet.BoolVar(&export, "export", false, "export existing filters")

	p.FlagSet.StringVar(&filtersFile, "filters-file", os.Getenv("GMAIL_FILTERS_FILE"), "Filters file (or env GMAIL_FILTERS_FILE)")
	p.FlagSet.StringVar(&labelsFile, "labels-file", os.Getenv("GMAIL_LABELS_FILE"), "Labels file (or env GMAIL_LABELS_FILE)")

	p.FlagSet.StringVar(&credsFile, "creds-file", os.Getenv("GMAIL_CREDENTIAL_FILE"), "Gmail credential file (or env var GMAIL_CREDENTIAL_FILE)")
	p.FlagSet.StringVar(&tokenFile, "token-file", filepath.Join(os.TempDir(), "token.json"), "Gmail oauth token file")

	// Set the before function.
	p.Before = func(ctx context.Context) error {
		// Set the log level.
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		if len(credsFile) < 1 {
			return errors.New("the Gmail credential file cannot be empty")
		}

		// Make sure the file exists.
		if _, err := os.Stat(credsFile); os.IsNotExist(err) {
			return fmt.Errorf("credential file %s does not exist", credsFile)
		}

		// Read the credentials file.
		b, err := ioutil.ReadFile(credsFile)
		if err != nil {
			return fmt.Errorf("reading client secret file %s failed: %v", credsFile, err)
		}

		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(b,
			// Manage labels.
			gmail.GmailLabelsScope,
			// Read, modify, and manage your settings.
			gmail.GmailSettingsBasicScope)
		if err != nil {
			return fmt.Errorf("parsing client secret file to config failed: %v", err)
		}

		// Get the client from the config.
		client, err := getClient(ctx, tokenFile, config)
		if err != nil {
			return fmt.Errorf("creating client failed: %v", err)
		}

		// Create the service for the Gmail client.
		api, err = gmail.New(client)
		if err != nil {
			return fmt.Errorf("creating Gmail client failed: %v", err)
		}

		return nil
	}

	p.Action = func(ctx context.Context, args []string) error {
		if filtersFile == "" {
			return errors.New("must set filters file location with --filters-file")
		}

		if labelsFile == "" {
			return errors.New("must set labels file location with --labels-file")
		}

		// On ^C, or SIGTERM handle exit.
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		signal.Notify(c, syscall.SIGTERM)
		go func() {
			for sig := range c {
				logrus.Infof("Received %s, exiting.", sig.String())
				os.Exit(0)
			}
		}()

		labels, err := getLabelMap()
		if err != nil {
			return err
		}

		if export {
			return exportExisting(labels, filtersFile, labelsFile)
		}

		fmt.Printf("Decoding filters from file %s\n", filtersFile)
		filters, err := decodeFiltersFile(filtersFile)
		if err != nil {
			return err
		}

		localLabels, err := decodeLabelsFile(labelsFile)
		if err != nil {
			return err
		}

		// Convert to gmail filters first in case this fails
		gmailFilters, err := convertToGmailFilters(filters, labels)
		if err != nil {
			return err
		}
		fmt.Printf("Converted %d local filters into %d gmail filters\n", len(filters), len(gmailFilters))

		// Delete our existing filters.
		if err := deleteExistingFilters(); err != nil {
			return err
		}

		// Add our gmail filters
		fmt.Printf("Adding %d gmail filters, this might take a bit...\n", len(gmailFilters))
		if err := addFilters(gmailFilters); err != nil {
			return err
		}

		fmt.Printf("Successfully updated %d filters\n", len(filters))

		if err := updateLabels(labels, localLabels); err != nil {
			return err
		}

		if err := writeLabelsToFile(labels, labelsFile); err != nil {
			return err
		}

		return nil
	}

	// Run our program.
	p.Run()
}
