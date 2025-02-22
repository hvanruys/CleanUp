package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"strconv"
)

func moveFilesToDateSubdirs(basepaths, filetemplates []string, startdates []int) error {
	// Ensure the lengths of filetemplates and startdates match
	if len(filetemplates) != len(startdates) {
		return fmt.Errorf("mismatch: filetemplates has %d entries, but startdates has %d", len(filetemplates), len(startdates))
	}

	// Compile regex patterns for each filetemplate
	regexPatterns := make([]*regexp.Regexp, len(filetemplates))
	for i, template := range filetemplates {
		// Convert glob-like patterns to regex (e.g., "*" to ".*")
		template := strings.ReplaceAll(regexp.QuoteMeta(template), `\.`, `.`)
		pattern := "^" + strings.ReplaceAll(template, `\*`, `.*`) + "$"
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern for template %s: %v", template, err)
		}
		regexPatterns[i] = re
	}

	
	// fmt.Println("Regex patterns:")
	// for i, re := range regexPatterns {
	// 	fmt.Printf("%d: %s\n", i, re)
	// }

	// Process each basepath
	for _, basepath := range basepaths {
		// Walk through the directory tree
		err := filepath.WalkDir(basepath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if d.IsDir() {
				return nil
			}

			filename := d.Name()
			matched := false
			var dateStr string

			// Check each template for a match
			for i, re := range regexPatterns {
				if re.MatchString(filename) {
					start := startdates[i]
					// Ensure the filename is long enough to extract the date
					if start+8 > len(filename) {
						fmt.Printf("Warning: Filename %s too short for date at position %d\n", filename, start)
						return nil // Skip this file
					}
					dateStr = filename[start : start+8]
					matched = true
					break
				}
			}	
			// If no template matches, delete the file
			if !matched {
				err := os.Remove(path)
				if err != nil {
					return fmt.Errorf("failed to delete unmatched file %s: %v", path, err)
				}
				fmt.Printf("Deleted unmatched file: %s\n", path)
				return nil
			}
			
			// Validate and parse the date string (YYYYMMDD)
			if len(dateStr) != 8 || !isValidDate(dateStr) {
				fmt.Printf("Warning: Invalid date %s in filename %s\n", dateStr, filename)
				return nil // Skip this file
			}

			year := dateStr[0:4]
			month := dateStr[4:6]
			day := dateStr[6:8]

			// Construct the new destination path
			newSubdir := filepath.Join(basepath, year, month, day)
			newPath := filepath.Join(newSubdir, filename)

			// Create the destination directory if it doesn't exist
			err = os.MkdirAll(newSubdir, 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %v", newSubdir, err)
			}

			// Move the file
			err = os.Rename(path, newPath)
			if err != nil {
				return fmt.Errorf("failed to move %s to %s: %v", path, newPath, err)
			}
			fmt.Printf("Moved %s to %s\n", filename, newSubdir)
			
			return nil
		})

		if err != nil {
			return fmt.Errorf("error walking directory %s: %v", basepath, err)
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

// Helper function to check if a string is numeric
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func main() {
	// Example usage with data from your YAML
	basepaths := []string{"/home/hugo/go-examples/received/E2H-MTG-LI"}
	filetemplates := []string{
		"W_XX-EUMETSAT*MTI1+LI-2-..-*BODY*",
  		"W_XX-EUMETSAT*MTI1+LI-2-..-*TRAIL*",
  		"W_XX-EUMETSAT*MTI1+LI-2-...-*BODY*",
  		"W_XX-EUMETSAT*MTI1+LI-2-...-*TRAIL*",
	}
	
	startdates := []int{97, 98, 98, 99}

	err := moveFilesToDateSubdirs(basepaths, filetemplates, startdates)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("File moving completed successfully.")
	}
}