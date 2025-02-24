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
/*
DeleteOldDirectories
--------------------
Problem Statement:
We have an array of strings called 'BasePaths' that contains the base paths. 
We want to delete the oldest subdirectories in the base paths until we reach 20% free space.
The subdirectories of the base paths are in the format YYYY/MM/DD.
Write a golang function called 'DeleteOldDirectories' that deletes the oldest directories in the base paths until we reach 20% free space.
There are only files in the DD directories. Sort the directories by YYYY , then MM, then DD (the oldest first).
When a YYYY directory is empty, it should be deleted. When an MM directory is empty, it should be deleted. 
When a DD directory is empty, it should be deleted.
The function should not have any arguments and should return an error.

*/
package main

import (
	"fmt"

	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
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

func DeleteOldDirectories() error {
    fmt.Println("Deleting old directories")
    // Collect all DD directories (format YYYY/MM/DD) from each base path.
    var directories []DirectoryInfo

    for _, basePath := range yamlconfig.BasePaths {
        // Construct a glob pattern for candidate DD directories.
        pattern := filepath.Join(basePath, "????", "??", "??")
        matches, err := filepath.Glob(pattern)
        if err != nil {
            return fmt.Errorf("failed to glob pattern %s: %v", pattern, err)
        }

        // Process the matched paths.
        for _, match := range matches {
            info, err := os.Stat(match)
            if err != nil || !info.IsDir() {
                continue
            }
            // Get the relative path from basePath; expected to be "YYYY/MM/DD"
            relPath, err := filepath.Rel(basePath, match)
            if err != nil {
                continue
            }
            parts := strings.Split(relPath, string(os.PathSeparator))
            if len(parts) != 3 {
                continue
            }
            year, month, day := parts[0], parts[1], parts[2]
            if len(year) != 4 || len(month) != 2 || len(day) != 2 {
                continue
            }
            // Build a date key, e.g. 20220225.
            //dateKey := year + month + day
            directories = append(directories, DirectoryInfo{
                Path:    match,
                ModTime: 0, // Not used for sorting; we sort by dateKey instead.
                // Reusing ModTime to store the numeric representation of date.
            })
            // In this example we do not directly store the dateKey;
            // instead, we will sort using the relative path string.
        }
    }

    // Sort the directories by their date extracted from the relative path.
    sort.Slice(directories, func(i, j int) bool {
        // Assume the directory paths are in the format base/ YYYY/MM/DD.
        // Extract the relative path and remove path separators.
        getDateKey := func(path string) string {
            // Find the position of the base directory among yamlconfig.BasePaths.
            // We assume the first matching basePath.
            for _, base := range yamlconfig.BasePaths {
                rel, err := filepath.Rel(base, path)
                if err == nil && rel != path {
                    // Remove path separators; "2022/02/25" becomes "20220225"
                    return strings.ReplaceAll(rel, string(os.PathSeparator), "")
                }
            }
            return path
        }
        return getDateKey(directories[i].Path) < getDateKey(directories[j].Path)
    })

    // Loop until free disk space is 20% or more.
    //for len(directories) > 0 {
        freeSpace, err := getFreeSpacePercentage()
        if err != nil {
            return fmt.Errorf("error getting free space: %v", err)
        }
        fmt.Printf("Current free space: %.2f%%\n", freeSpace)
		
        if freeSpace >= 20.0 {
            //break
        }

        // Delete the oldest directory (first in the sorted slice).
        oldestDir := directories[0]
        fmt.Printf("Deleting directory: %s\n", oldestDir.Path)
        //if err := os.RemoveAll(oldestDir.Path); err != nil {
        //    return fmt.Errorf("error deleting directory %s: %v", oldestDir.Path, err)
        //}

        // After deleting the DD directory, attempt to clean up empty parent directories.
        cleanUpEmptyAncestors(oldestDir.Path)

        // Remove the deleted directory from our slice.
        directories = directories[1:]
    //}

    return nil
}

// cleanUpEmptyAncestors deletes empty parent directories
// (e.g. the MM and YYYY directories) up to the corresponding BasePath.
func cleanUpEmptyAncestors(deletedPath string) {
    // Walk upward from the deleted directory.
    dir := filepath.Dir(deletedPath)
    for {
        // List entries in the current directory.
        entries, err := os.ReadDir(dir)
        if err != nil {
            break
        }
        // If directory is not empty, or if it is one of the base paths, stop.
        if len(entries) > 0 {
            break
        }
        // Delete the empty directory.
        if err := os.Remove(dir); err != nil {
            break
        }
        // Get the new parent.
        parent := filepath.Dir(dir)
        // Stop if we've reached a base path (or root).
        if parent == dir {
            break
        }
        dir = parent
    }
}

// getFreeSpacePercentage returns the percentage of free disk space.
func getFreeSpacePercentage() (float64, error) {
    var stat syscall.Statfs_t
    // Change "/" to a desired mount point if needed.
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

	// err = moveFilesToDateSubdirs()
	// if err != nil {
	// 	fmt.Printf("Error: %v\n", err)
	// } else {
	// 	fmt.Println("File moving completed successfully.")
	// }

	err = DeleteOldDirectories()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Delete old directories completed successfully.")
	}

	// fmt.Println("Starting event scheduler...")

	// // Channel to signal goroutines to stop
	// done := make(chan bool)

	// // Start goroutines for each event
	// go eventDeleteOldDirs(done)
	// go eventMoveFiles(done)

	// select {}

}
