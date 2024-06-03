package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// Constants
	DEFAULT_LIQUIBASE_VERSION = "4.21.1"
	LIQUIBASE_ZIP_URL         = "https://github.com/liquibase/liquibase/releases/download/v4.21.1/liquibase-4.21.1.zip"
	LIQUIBASE_ZIP_FILE        = "liquibase-4.21.1.zip"
	LIQUIBASE_DIR             = "liquibase-4.21.1"
	LIQUIBASE_EXT_URL         = "https://github.com/liquibase/{ext}/releases/download/{extVersion}/{extVersion2}.jar"
)

// Liquibase extensions list as a variable
var LIQUIBASE_EXT_LIST = []string{"liquibase-bigquery", "liquibase-redshift"}

// GoLiquibase struct
type GoLiquibase struct {
	DefaultsFile            string
	LiquibaseHubMode        string
	LogLevel                string
	LiquibaseDir            string
	JdbcDriversDir          string
	AdditionalClasspath     string
	Version                 string
	LiquibaseLibDir         string
	LiquibaseInternalDir    string
	LiquibaseInternalLibDir string
	Args                    []string
}

// NewGoLiquibase creates a new GoLiquibase instance
func NewGoLiquibase(defaultsFile, liquibaseHubMode, logLevel, liquibaseDir, jdbcDriversDir, additionalClasspath, version string) *GoLiquibase {
	return &GoLiquibase{
		DefaultsFile:            defaultsFile,
		LiquibaseHubMode:        liquibaseHubMode,
		LogLevel:                logLevel,
		LiquibaseDir:            liquibaseDir,
		JdbcDriversDir:          jdbcDriversDir,
		AdditionalClasspath:     additionalClasspath,
		Version:                 version,
		LiquibaseLibDir:         filepath.Join(liquibaseDir, "lib"),
		LiquibaseInternalDir:    filepath.Join(liquibaseDir, "internal"),
		LiquibaseInternalLibDir: filepath.Join(liquibaseDir, "internal", "lib"),
	}
}

// Initialize the GoLiquibase instance
func (pl *GoLiquibase) Initialize() error {
	if pl.DefaultsFile != "" {
		if !fileExists(pl.DefaultsFile) {
			return fmt.Errorf("defaultsFile not found! %s", pl.DefaultsFile)
		}
		pl.Args = append(pl.Args, fmt.Sprintf("--defaults-file=%s", pl.DefaultsFile))
	}

	if pl.LiquibaseHubMode != "" {
		pl.Args = append(pl.Args, fmt.Sprintf("--hub-mode=%s", pl.LiquibaseHubMode))
	}

	if pl.LogLevel != "" {
		pl.Args = append(pl.Args, fmt.Sprintf("--log-level=%s", pl.LogLevel))
	}

	// If liquibaseDir is provided, use it
	if pl.LiquibaseDir != "" {
		pl.Version = "user-provided"
	} else {
		// Download and extract liquibase if it doesn't exist
		if err := pl.DownloadLiquibase(); err != nil {
			return err
		}
	}

	// Download additional java libraries
	if err := pl.DownloadLiquibaseExtensionLibs(); err != nil {
		return err
	}

	return nil
}

// Execute the Liquibase command with arguments
func (pl *GoLiquibase) Execute(arguments ...string) error {
	cmdArgs := append(pl.Args, arguments...)
	cmd := exec.Command(filepath.Join(pl.LiquibaseDir, "liquibase"), cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Current working dir is %s", os.Getenv("PWD"))
	log.Printf("Executing liquibase %s", strings.Join(cmdArgs, " "))

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute liquibase command: %v", err)
	}

	return nil
}

// Add an argument to the command
func (pl *GoLiquibase) AddArg(key, val string) {
	pl.Args = append(pl.Args, fmt.Sprintf("--%s=%s", key, val))
}

// Update the database
func (pl *GoLiquibase) Update() error {
	return pl.Execute("update")
}

// Update the database with SQL statements
func (pl *GoLiquibase) UpdateSQL() error {
	return pl.Execute("updateSQL")
}

// Update to a specific tag
func (pl *GoLiquibase) UpdateToTag(tag string) error {
	log.Printf("Updating to tag: %s", tag)
	return pl.Execute("update-to-tag", tag)
}

// Validate the database schema
func (pl *GoLiquibase) Validate() error {
	return pl.Execute("validate")
}

// Show the current status of the database
func (pl *GoLiquibase) Status() error {
	return pl.Execute("status")
}

// Rollback the database to a specific tag
func (pl *GoLiquibase) Rollback(tag string) error {
	log.Printf("Rolling back to tag: %s", tag)
	return pl.Execute("rollback", tag)
}

// Rollback the database to a specific datetime
func (pl *GoLiquibase) RollbackToDatetime(datetime string) error {
	log.Printf("Rolling back to %s", datetime)
	return pl.Execute("rollbackToDate", datetime)
}

// Sync the changelog with the database
func (pl *GoLiquibase) ChangelogSync() error {
	log.Println("Marking all undeployed changes as executed in database.")
	return pl.Execute("changelog-sync")
}

// Sync the changelog with the database up to a specific tag
func (pl *GoLiquibase) ChangelogSyncToTag(tag string) error {
	log.Printf("Marking all undeployed changes as executed up to tag %s in database.", tag)
	return pl.Execute("changelog-sync-to-tag", tag)
}

// Clear checksums in the database
func (pl *GoLiquibase) ClearChecksums() error {
	log.Println("Clearing checksums in database.")
	return pl.Execute("clear-checksums")
}

// Release locks in the database
func (pl *GoLiquibase) ReleaseLocks() error {
	log.Println("Releasing locks in database.")
	return pl.Execute("release-locks")
}

// Download Liquibase from Github and extract it
func (pl *GoLiquibase) DownloadLiquibase() error {
	if fileExists(pl.LiquibaseDir) {
		log.Printf("Liquibase version %s found, skipping download...", pl.Version)
		return nil
	}
	zipFilePath := filepath.Join(os.TempDir(), LIQUIBASE_ZIP_FILE)
	if err := pl.downloadFile(LIQUIBASE_ZIP_URL, zipFilePath); err != nil {
		return err
	}

	log.Printf("Extracting Liquibase to %s", pl.LiquibaseDir)
	if err := unzipFile(zipFilePath, pl.LiquibaseDir); err != nil {
		return err
	}

	os.Remove(zipFilePath)
	return nil
}

// Download Liquibase extension libraries
func (pl *GoLiquibase) DownloadLiquibaseExtensionLibs() error {
	for _, ext := range LIQUIBASE_EXT_LIST {
		extVersion := fmt.Sprintf("%s-%s", ext, pl.Version)
		extVersion2 := fmt.Sprintf("v%s", pl.Version)
		extURL := LIQUIBASE_EXT_URL
		extURL = strings.ReplaceAll(extURL, "{ext}", ext)
		extURL = strings.ReplaceAll(extURL, "{extVersion}", extVersion)
		extURL = strings.ReplaceAll(extURL, "{extVersion2}", extVersion2)

		err := pl.downloadAdditionalJavaLibrary(extURL, pl.LiquibaseLibDir)
		if err != nil {
			log.Printf("Failed to download Liquibase extension: %s", extVersion)
		}
	}
	return nil
}

// Download a file from a given URL
func (pl *GoLiquibase) downloadFile(url, destination string) error {
	log.Printf("Downloading %s to %s", url, destination)
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading file: %s", response.Status)
	}

	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	return err
}

// Download an additional java library
func (pl *GoLiquibase) downloadAdditionalJavaLibrary(downloadURL, destinationDir string) error {
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return err
	}

	libFileName := filepath.Base(parsedURL.Path)
	if !strings.HasSuffix(libFileName, ".zip") && !strings.HasSuffix(libFileName, ".jar") {
		return fmt.Errorf("unexpected URL. Expecting link to a **.jar** or **.zip** file")
	}

	destinationFile := filepath.Join(destinationDir, libFileName)

	if fileExists(destinationFile) {
		log.Printf("Java lib already available, skipping download: %s", destinationFile)
		return nil
	}

	log.Printf("Downloading java lib: %s to %s", downloadURL, destinationFile)
	return pl.downloadFile(downloadURL, destinationFile)
}

// Check if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Unzip a zip file
func unzipFile(zipFilePath, destinationDir string) error {
	reader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		filePath := filepath.Join(destinationDir, file.Name)

		// Check for directory creation
		if !file.FileInfo().IsDir() {
			// Extract the file
			fileReader, err := file.Open()
			if err != nil {
				return err
			}
			defer fileReader.Close()

			// Create the file
			os.MkdirAll(filepath.Dir(filePath), 0755)

			// Write the file to the destination
			fileWriter, err := os.Create(filePath)
			if err != nil {
				return err
			}
			defer fileWriter.Close()

			_, err = io.Copy(fileWriter, fileReader)
			if err != nil {
				return err
			}
		} else {
			// Create the directory
			os.MkdirAll(filePath, 0755)
		}
	}

	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "goliquibase",
		Short: "A Go implementation of GoLiquibase",
		Run: func(cmd *cobra.Command, args []string) {
			defaultsFile, _ := cmd.Flags().GetString("defaultsFile")
			liquibaseHubMode, _ := cmd.Flags().GetString("liquibaseHubMode")
			logLevel, _ := cmd.Flags().GetString("logLevel")
			liquibaseDir, _ := cmd.Flags().GetString("liquibaseDir")
			jdbcDriversDir, _ := cmd.Flags().GetString("jdbcDriversDir")
			additionalClasspath, _ := cmd.Flags().GetString("additionalClasspath")
			version, _ := cmd.Flags().GetString("version")

			pl := NewGoLiquibase(
				defaultsFile,
				liquibaseHubMode,
				logLevel,
				liquibaseDir,
				jdbcDriversDir,
				additionalClasspath,
				version,
			)

			if err := pl.Initialize(); err != nil {
				log.Fatal(err)
			}

			// Parse and handle arguments
			if err := pl.Execute(args...); err != nil {
				log.Fatal(err)
			}
		},
	}

	rootCmd.Flags().StringP("defaultsFile", "d", "liquibase.properties", "Relative path to liquibase.properties file")
	rootCmd.Flags().StringP("liquibaseHubMode", "h", "off", "Liquibase Hub Mode default 'off'")
	rootCmd.Flags().StringP("logLevel", "l", "", "Log level name")
	rootCmd.Flags().StringP("liquibaseDir", "D", "", "User provided Liquibase directory")
	rootCmd.Flags().StringP("jdbcDriversDir", "j", "", "User provided JDBC drivers directory. All jar files under this directory are loaded")
	rootCmd.Flags().StringP("additionalClasspath", "a", "", "Additional classpath to import java libraries and Liquibase extensions")
	rootCmd.Flags().StringP("version", "v", DEFAULT_LIQUIBASE_VERSION, "Liquibase version")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
