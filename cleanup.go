/*
moveFilesToDateSubdirs
----------------------
Problem Statement:
We have the following global object :
-BasePaths: array of strings
-FileTemplates: array of strings
-StartDates: array of integers

We have an array of strings called 'BasePaths' that contains the paths of the files we want to move.
We have an array of strings called 'FileTemplates' that contains the templates of the filenames we want to move.
We have an array of int's called 'StartDates' that contains the exact position of the date substring of the filenames.
The date substring of the filenames is always 8 characters long.
The strings in 'FileTemplates' are regular expressions.
The strings in 'FileTemplates' are glob-like patterns.
The strings in 'FileTemplates' are case-sensitive.

We want to move all the files in the paths of 'BasePaths' to a new subdirectory of the paths with a layout YYYY/MM/DD.
Write a golang function called 'moveFilesToDateSubdirs' that moves the files to the new subdirectories. When the filename does not match the template,
the file should be deleted. The function should not have any arguments and should return an error. The function should return an error if the length of
'FileTemplates' and 'StartDates' do not match. The function should return an error if the date substring of the filename is not valid.
Note: The function should be able to handle any number of files and directories.
*/
package main

import (
	"fmt"

	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Define structs to match the YAML structure
type YAMLConfig struct {
	FileTemplates  []string `yaml:"filetemplates"`
	StartDates     []int    `yaml:"startdates"`
	BasePaths      []string `yaml:"basepaths"`
	Disks          []string `yaml:"disks"`
	FreeDiskSpaces []int    `yaml:"freediskspaces"`
}

var yamlconfig YAMLConfig
// Global slice of compiled regexps
var regexPatterns []*regexp.Regexp

// Function for the 5-second event
func eventMoveFiles(done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			fmt.Println("Event 1: Executing every 10 seconds")
			moveFilesToDateSubdirs()
			time.Sleep(10 * time.Second)
		}
	}
}

// Function for the 10-second event
func eventDeleteOldDirs(done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			fmt.Println("Event 2: Executing every 30 seconds")
			time.Sleep(30 * time.Second)
		}
	}
}

func moveFilesToDateSubdirs() error {

	fmt.Printf("Moving files to date subdirectories\n")

	if len(	regexPatterns) == 0 {
	}
		// Compile regex patterns for each filetemplate
	// Process each base path
	for _, basepath := range yamlconfig.BasePaths {
		entries, err := os.ReadDir(basepath)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %v", basepath, err)
		}

		for _, entry := range entries {
			// Process only files (skip directories)
			if entry.IsDir() {
				continue
			}

			filename := entry.Name()
			matched := false
			var dateStr string

			// Check each template for a match
			for i, re := range regexPatterns {
				if re.MatchString(filename) {
					start := yamlconfig.StartDates[i]
					// Check if filename is long enough to extract the date substring
					if start+8 > len(filename) {
						fmt.Printf("Warning: Filename %s too short for date at position %d\n", filename, start)
						matched = false
						break
					}
					dateStr = filename[start : start+8]
					matched = true
					break
				}
			}

			fullPath := filepath.Join(basepath, filename)
			// If no template matches, delete the file
			if !matched {
				if err := os.Remove(fullPath); err != nil {
					return fmt.Errorf("failed to delete unmatched file %s: %v", fullPath, err)
				}
				fmt.Printf("Deleted unmatched file: %s\n", fullPath)
				continue
			}

			// Validate the date substring (YYYYMMDD)
			if len(dateStr) != 8 || !isValidDate(dateStr) {
				fmt.Printf("Warning: Invalid date %s in filename %s\n", dateStr, filename)
				continue // Skip the file if date is invalid
			}

			year := dateStr[0:4]
			month := dateStr[4:6]
			day := dateStr[6:8]

			// Construct the new subdirectory path: basepath/YYYY/MM/DD
			newSubdir := filepath.Join(basepath, year, month, day)
			newPath := filepath.Join(newSubdir, filename)

			// Create the destination directory if it does not exist
			if err := os.MkdirAll(newSubdir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", newSubdir, err)
			}

			// Move the file to the new destination
			if err := os.Rename(fullPath, newPath); err != nil {
				return fmt.Errorf("failed to move %s to %s: %v", fullPath, newPath, err)
			}
			fmt.Printf("Moved %s to %s\n", filename, newSubdir)

		}
	}
	return nil
}

// Helper function to validate YYYYMMDD date string
func isValidDate(dateStr string) bool {
	if len(dateStr) != 8 {
		return false
	}
	year := dateStr[0:4]
	month := dateStr[4:6]
	day := dateStr[6:8]

	// Basic checks (could be enhanced with time.Parse for stricter validation)
	if !isNumeric(year) || !isNumeric(month) || !isNumeric(day) {
		return false
	}
	m, _ := strconv.Atoi(month)
	d, _ := strconv.Atoi(day)
	return m >= 1 && m <= 12 && d >= 1 && d <= 31 // Simplified; ignores month-specific days
}

// DirectoryInfo holds information about a directory
type DirectoryInfo struct {
	Path    string
	ModTime int64
}

func DeleteOldDirectories(basePaths []string) error {
	fmt.Printf("Deleting old directories in %v\n", basePaths)

	// Regular expression for YYYY/MM/DD pattern
	datePattern := regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)

	// Collect all matching directories
	var directories []DirectoryInfo
	for _, basePath := range basePaths {
		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Get relative path from base
			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				return err
			}

			// Check if directory matches YYYY/MM/DD pattern
			if info.IsDir() && datePattern.MatchString(relPath) {
				directories = append(directories, DirectoryInfo{
					Path:    path,
					ModTime: info.ModTime().Unix(),
				})
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error walking path %s: %v", basePath, err)
		}
	}

	// Sort directories by modification time (oldest first)
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].ModTime < directories[j].ModTime
	})

	// Keep deleting oldest directories until we reach 20% free space
	for len(directories) > 0 {
		freeSpace, err := getFreeSpacePercentage()
		if err != nil {
			return fmt.Errorf("error getting free space: %v", err)
		}

		if freeSpace >= 20.0 {
			break
		}

		// Delete oldest directory
		oldestDir := directories[0]
		directories = directories[1:]

		err = os.RemoveAll(oldestDir.Path)
		if err != nil {
			return fmt.Errorf("error deleting directory %s: %v", oldestDir.Path, err)
		}
	}

	return nil
}

// getFreeSpacePercentage returns the percentage of free disk space
func getFreeSpacePercentage() (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, err
	}

	totalSpace := float64(stat.Blocks) * float64(stat.Bsize)
	freeSpace := float64(stat.Bavail) * float64(stat.Bsize)

	return (freeSpace / totalSpace) * 100, nil
}

// isNumeric checks if a string is purely numeric
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// func main() {
// 	// Example basepaths from your earlier YAML
// 	basepaths := []string{
// 		"/mnt/nfs_Vol3T/received/hvs-2/E2H-MTG-LI",
// 	}

// 	err := DeleteOldDirectories(basepaths)
// 	if err != nil {
// 		log.Fatalf("Error: %v", err)
// 	}
// 	log.Println("Directory cleanup completed successfully.")
// }

func main() {

	data, err := os.ReadFile("directories.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Parse the YAML content

	yaml.Unmarshal(data, &yamlconfig)

	if err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	// Print the parsed content
	fmt.Println("File Templates:")
	for i, template := range yamlconfig.FileTemplates {
		fmt.Printf("  %d: %s\n", i+1, template)
	}

	fmt.Println("\nStart Dates:")
	for i, date := range yamlconfig.StartDates {
		fmt.Printf("  %d: %d\n", i+1, date)
	}

	fmt.Println("\nBase Paths:")
	for i, path := range yamlconfig.BasePaths {
		fmt.Printf("  %d: %s\n", i+1, path)
	}

	fmt.Println("\nDisks:")
	for i, disk := range yamlconfig.Disks {
		fmt.Printf("  %d: %s\n", i+1, disk)
	}

	fmt.Println("\nFree Disk Spaces:")
	for i, space := range yamlconfig.FreeDiskSpaces {
		fmt.Printf("  %d: %d\n", i+1, space)
	}

	// Ensure the lengths of FileTemplates and StartDates match
	if len(yamlconfig.FileTemplates) != len(yamlconfig.StartDates) {
		fmt.Printf("mismatch: filetemplates has %d entries, but startdates has %d", len(yamlconfig.FileTemplates), len(yamlconfig.StartDates))
		return
	}

	// Compile regex patterns for each filetemplate
	regexPatterns = make([]*regexp.Regexp, 0, len(yamlconfig.FileTemplates))
	for _, template := range yamlconfig.FileTemplates {
		
		// Convert glob-like patterns to regex (replace "*" with ".*")
		escaped := regexp.QuoteMeta(template)
		escaped = strings.ReplaceAll(escaped, `\.`, `.`)
		pattern := "^" + strings.ReplaceAll(escaped, `\*`, `.*`) + "$"
		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Printf("Warning: Skipping invalid pattern %s: %v\n", template, err)
			continue
		}
		regexPatterns = append(regexPatterns, re)
		
	}

	for i := range regexPatterns {
		fmt.Printf("%s\n", regexPatterns[i].String())
	}

	err = moveFilesToDateSubdirs()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("File moving completed successfully.")
	}

	fmt.Println("Starting event scheduler...")

	// Channel to signal goroutines to stop
	done := make(chan bool)

	// Start goroutines for each event
	go eventDeleteOldDirs(done)
	go eventMoveFiles(done)

	// Let the events run for 30 seconds
	//time.Sleep(30 * time.Second)
	select {}
	// Signal goroutines to stop
	// close(done)

	// // Give a moment for goroutines to exit cleanly
	// time.Sleep(1 * time.Second)

	// fmt.Println("Program terminated.")

}
