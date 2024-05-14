// --------------------------------------------------------------------------------
// Author: Thomas F McGeehan V
//
// This file is part of a software project developed by Thomas F McGeehan V.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// For more information about the MIT License, please visit:
// https://opensource.org/licenses/MIT
//
// Acknowledgment appreciated but not required.
// --------------------------------------------------------------------------------

package main

import (
	"archive/zip"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	defaultLiquibaseVersion = "4.21.1"
	liquibaseZipURL         = "https://github.com/liquibase/liquibase/releases/download/v4.21.1/liquibase-4.21.1.zip"
	liquibaseDir            = "liquibase-4.21.1"
)

func main() {
	defaultsFile := flag.String("defaultsFile", "liquibase.properties", "Relative path to liquibase.properties file")
	liquibaseHubMode := flag.String("liquibaseHubMode", "off", "Liquibase Hub Mode")
	logLevel := flag.String("logLevel", "", "Liquibase log level")
	flag.Parse()

	executeLiquibase(*defaultsFile, *liquibaseHubMode, *logLevel)
}

func executeLiquibase(defaultsFile, liquibaseHubMode, logLevel string) {
	if _, err := os.Stat(liquibaseDir); os.IsNotExist(err) {
		downloadAndExtractLiquibase(liquibaseZipURL, liquibaseDir)
	}

	args := []string{
		"--defaultsFile=" + defaultsFile,
		"--hubMode=" + liquibaseHubMode,
	}
	if logLevel != "" {
		args = append(args, "--logLevel="+logLevel)
	}

	cmd := exec.Command("java", append([]string{"-jar", liquibaseDir + "/liquibase.jar"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Liquibase command failed: %v", err)
	}
}

func downloadAndExtractLiquibase(url, destDir string) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status: %s", resp.Status)
	}

	tmpZipFile, err := os.CreateTemp("", "liquibase-*.zip")
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpZipFile.Name())

	if _, err := io.Copy(tmpZipFile, resp.Body); err != nil {
		log.Fatalf("Failed to copy to temp file: %v", err)
	}

	if err := unzip(tmpZipFile.Name(), destDir); err != nil {
		log.Fatalf("Failed to unzip file: %v", err)
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to handle error checking
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
