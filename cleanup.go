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
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"gopkg.in/yaml.v3"
)

type StructTemplate struct {
	FileTemplate string `yaml:"filetemplate"`
	StartDate    int    `yaml:"startdate"`
	DateLayout   string `yaml:"datelayout"`
}

type StructDisks struct {
	DiskName      string `yaml:"diskname"`
	FreeDiskSpace int    `yaml:"freediskspace"`
}

type YAMLConfig struct {
	FileTemplates []StructTemplate `yaml:"filetemplates"`
	BasePaths     []string         `yaml:"basepaths"`
	Disks         []StructDisks    `yaml:"disks"`
	PortNumber    string           `yaml:"portnumber"`
}

var yamlconfig YAMLConfig
var regexPatterns []*regexp.Regexp
var portnumber = "7000"

// SystemMetrics represents CPU and Disk data for sending to the client
type SystemMetrics struct {
	CoreUsages  []float64 `json:"core_usages"` // Percentage for each core
	DiskUsed    []float64 `json:"disks_used"`  // Percentage of disk space used
	DiskFree    []float64 `json:"disks_free"`  // Percentage of disk space free
	Timestamp   int64     `json:"timestamp"`   // Unix timestamp in milliseconds
	DiskLabel   []string  `json:"disks_label"` // Label for disk
	DiskTotal   []uint64  `json:"disks_total"` // Total disk space
	AvailDirs   []string  `json:"avail_dirs"`
	MemoryUsed  float64   `json:"memory_used"`  // Percentage of memory used
	MemoryFree  float64   `json:"memory_free"`  // Percentage of memory free
	MemoryTotal uint64    `json:"memory_total"` // Total memory in MB

}

// Global variables
var (
	clients = make(map[*websocket.Conn]bool)
	//broadcast = make(chan SystemMetrics)
	mutex sync.Mutex
)

// Function for the 10-second event
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

// Function for the 30-minutes event
func eventDeleteOldDirs(done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			fmt.Println("Event 2: Executing every 30 minutes")
			deleteOldDirectories()
			time.Sleep(30 * time.Minute)
		}
	}
}

func convertYYYYDDDToYYYYMMDD(yyyydoy string) (string, error) {
	if len(yyyydoy) != 7 {
		return "", fmt.Errorf("invalid input length: expected 7 characters, got %d", len(yyyydoy))
	}

	yearStr := yyyydoy[0:4]
	doyStr := yyyydoy[4:7]

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return "", fmt.Errorf("invalid year: %v", err)
	}
	doy, err := strconv.Atoi(doyStr)
	if err != nil {
		return "", fmt.Errorf("invalid day-of-year: %v", err)
	}

	// Get the date for January 1st of the given year.
	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	// Add (doy-1) days to get the desired date.
	date := startOfYear.AddDate(0, 0, doy-1)
	return date.Format("20060102"), nil
}

func moveFilesToDateSubdirs() error {

	fmt.Printf("Moving files to date subdirectories\n")

	if len(regexPatterns) == 0 {
		return fmt.Errorf("regexPatterns is empty")
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
			var datelayout string

			// Check each template for a match
			for i, re := range regexPatterns {
				if re.MatchString(filename) {
					start := yamlconfig.FileTemplates[i].StartDate
					// Check if filename is long enough to extract the date substring
					if yamlconfig.FileTemplates[i].DateLayout == "YYYYMMDD" {
						datelayout = "YYYYMMDD"
						if start+8 > len(filename) {
							fmt.Printf("Warning: Filename %s too short for date at position %d\n", filename, start)
							matched = false
							break
						}
						dateStr = filename[start : start+8]
						matched = true
					} else if yamlconfig.FileTemplates[i].DateLayout == "YYYYDDD" {
						datelayout = "YYYYDDD"
						if start+7 > len(filename) {
							fmt.Printf("Warning: Filename %s too short for date at position %d\n", filename, start)
							matched = false
							break
						}
						dateStr = filename[start : start+7]
						matched = true
					} else {
						return fmt.Errorf("DateLayout is not YYYYMMDD or YYYYDDD")
					}
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
			var year, month, day string
			if datelayout == "YYYYMMDD" {
				if len(dateStr) != 8 || !isValidDate(dateStr) {
					fmt.Printf("Warning: Invalid date %s in filename %s\n", dateStr, filename)
					continue // Skip the file if date is invalid
				}
				year = dateStr[0:4]
				month = dateStr[4:6]
				day = dateStr[6:8]

			} else {
				if len(dateStr) != 7 {
					fmt.Printf("Warning: Invalid date %s in filename %s\n", dateStr, filename)
					continue // Skip the file if date is invalid
				}
				convertedDate, err := convertYYYYDDDToYYYYMMDD(dateStr)
				if err != nil {
					fmt.Printf("Warning: Failed to convert date %s: %v\n", dateStr, err)
					continue
				}
				year = convertedDate[0:4]
				month = convertedDate[4:6]
				day = convertedDate[6:8]
			}

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

func deleteOldDirectories() error {
	fmt.Println("Deleting old directories")
	// Collect all DD directories (format YYYY/MM/DD) from each base path.
	var directories []DirectoryInfo
	var requiredfreediskspace int
	requiredfreediskspace = 20

	for _, thedisk := range yamlconfig.Disks {

		requiredfreediskspace = thedisk.FreeDiskSpace
		freeSpace, err := getFreeSpacePercentage(thedisk.DiskName)
		directories = []DirectoryInfo{}

		fmt.Printf("Disk: %s free space %f required:%d \n", thedisk.DiskName, freeSpace, requiredfreediskspace)
		fmt.Print("=======================================================\n")
		if err != nil {
			return fmt.Errorf("error getting free space: %v", err)
		}
		if int(math.Round(freeSpace)) >= requiredfreediskspace {
			continue
		}

		for {

			directories = []DirectoryInfo{}

			for _, basePath := range yamlconfig.BasePaths {
				if !strings.Contains(basePath, thedisk.DiskName) {
					continue
				}

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
					dateKey := year + month + day
					dateKeyInt, _ := strconv.ParseInt(dateKey, 10, 64)
					directories = append(directories, DirectoryInfo{
						Path:    match,
						ModTime: dateKeyInt,
					})
				}

				}

			// Check if there are any directories to delete
			if len(directories) == 0 {
				fmt.Printf("No more directories to delete for disk %s, but free space is still below required (%d%%)\n",
					thedisk.DiskName, requiredfreediskspace)
				break
			}

			// Sort the directories by their date extracted from the relative path.
			sort.Slice(directories, func(i, j int) bool {
				return directories[i].ModTime < directories[j].ModTime
			})

			// Delete the oldest directory (first in the sorted slice).
			oldestDir := directories[0]
			fmt.Printf("Deleting directory: %s\n", oldestDir.Path)
			if err := os.RemoveAll(oldestDir.Path); err != nil {
				return fmt.Errorf("error deleting directory %s: %v", oldestDir.Path, err)
			}

			// After deleting the DD directory, attempt to clean up empty parent directories.
			cleanUpEmptyAncestors(oldestDir.Path)

			// Check if we've reached the required free space
			freeSpace, err = getFreeSpacePercentage(thedisk.DiskName)
			if err != nil {
				return fmt.Errorf("error getting free space: %v", err)
			}
			if int(math.Round(freeSpace)) >= requiredfreediskspace {
				fmt.Printf("Reached required free space (%.2f%%) for disk %s\n", freeSpace, thedisk.DiskName)
				break
			}
		}

	}

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
func getFreeSpacePercentage(diskdir string) (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(diskdir, &stat)
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

func main() {

	data, err := os.ReadFile("directories.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
		return
	}

	// Parse the YAML content

	err = yaml.Unmarshal(data, &yamlconfig)
	if err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
		return
	}

	// Print the parsed content
	fmt.Println("File Templates:")
	for i, template := range yamlconfig.FileTemplates {
		fmt.Printf("  %d: %s %d %s\n", i+1, template.FileTemplate, template.StartDate, template.DateLayout)
	}

	// Compile regex patterns for each filetemplate
	regexPatterns = make([]*regexp.Regexp, 0, len(yamlconfig.FileTemplates))
	for _, template := range yamlconfig.FileTemplates {

		// Convert glob-like patterns to regex (replace "*" with ".*")
		escaped := regexp.QuoteMeta(template.FileTemplate)
		escaped = strings.ReplaceAll(escaped, `\.`, `.`)
		pattern := "^" + strings.ReplaceAll(escaped, `\*`, `.*`) + "$"
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Error: invalid pattern %s: %v\n", template.FileTemplate, err)
			return
		}
		regexPatterns = append(regexPatterns, re)

	}

	// for i := range regexPatterns {
	// 	fmt.Printf("%s\n", regexPatterns[i].String())
	// }

	ips, err := GetLocalIPs()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ips)

	err = moveFilesToDateSubdirs()
	if err != nil {
		fmt.Printf("Error moveFilesToDateSubdirs: %v\n", err)
	}

	err = deleteOldDirectories()
	if err != nil {
		fmt.Printf("Error DeleteOldDirectories: %v\n", err)
	}

	fmt.Println("Starting event scheduler...")

	// Channel to signal goroutines to stop
	done := make(chan bool)

	// Start goroutines for each event
	go eventDeleteOldDirs(done)
	go eventMoveFiles(done)

	//	select {}

	// Start collecting and broadcasting metrics
	go collectMetrics()

	// WebSocket handler for real-time updates
	http.HandleFunc("/ws", handleWebSocket)

	// Serve the HTML page
	http.HandleFunc("/", serveIndex)

	// Serve static files (e.g., Plotly.js)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Register /disks endpoint to list available hard disks
	http.HandleFunc("/disks", diskListHandler)

	log.Println("Server starting on :" + portnumber + "...")
	log.Fatal(http.ListenAndServe(":"+portnumber, nil))

}

func collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Keep the last 100 data points for each core
	const maxPoints = 100
	var coreData = make(map[int][]float64) // Maps core index to usage history
	var timestamps []int64                 // Shared timestamps for all cores

	counter := 60
	var availdirs []string

	for range ticker.C {
		counter++

		var diskused []float64
		var diskfree []float64
		var disktotal []uint64
		var disklabels []string

		// Execute checkDateDirs every 10 seconds
		if counter >= 60 {
			availdirs = nil
			for _, basePath := range yamlconfig.BasePaths {
				//fmt.Printf("Checking date directories... %s\n", basePath)

				thedirstring, err := constructDirString(basePath)
				if err != nil {
					fmt.Printf("Error checking directories: %v\n", err)
				}

				availdirs = append(availdirs, thedirstring)
			}
			// Reset the counter
			counter = 0
		}
		// Get CPU usage per core
		usages, err := cpu.Percent(0, true) // Get per-core CPU usage, 0 for instantaneous
		if err != nil {
			log.Printf("Error getting CPU usage: %v", err)
			continue
		}

		for _, disk := range yamlconfig.Disks {
			diskUsage, err := getDiskUsage(disk.DiskName)
			if err != nil {
				log.Printf("Error getting disk usage: %v", err)
				continue
			}
			diskused = append(diskused, diskUsage.UsedPercent)
			disktotal = append(disktotal, diskUsage.Total/1024/1024/1024)
			diskfree = append(diskfree, 100-diskUsage.UsedPercent)
			disklabels = append(disklabels, disk.DiskName)

		}

		// Get memory statistics
		memUsed, memFree, memTotal, err := getMemoryStats()
		if err != nil {
			log.Printf("Error getting memory stats: %v", err)
			continue
		}

		now := time.Now().UnixMilli()
		metrics := SystemMetrics{
			CoreUsages: make([]float64, len(usages)),
			DiskUsed:   make([]float64, len(diskused)),
			DiskFree:   make([]float64, len(diskfree)),
			Timestamp:  now,
			DiskLabel:  make([]string, len(disklabels)),
			DiskTotal:  make([]uint64, len(disktotal)),
			AvailDirs:  make([]string, len(availdirs)),
			MemoryUsed:  memUsed,
            MemoryFree:  memFree,
            MemoryTotal: memTotal,
		}

		// Copy current CPU usage to metrics
		copy(metrics.CoreUsages, usages)    // Usage in percentage (0-100)
		copy(metrics.DiskUsed, diskused)    // Usage in percentage (0-100)
		copy(metrics.DiskFree, diskfree)    // Usage in percentage (0-100)
		copy(metrics.DiskLabel, disklabels) // Usage in percentage (0-100)
		copy(metrics.DiskTotal, disktotal)  // Usage in percentage (0-100)
		copy(metrics.AvailDirs, availdirs)


		// Add new data to history
		mutex.Lock()
		timestamps = append(timestamps, now)
		for i := range metrics.CoreUsages {
			coreData[i] = append(coreData[i], metrics.CoreUsages[i])
			if len(coreData[i]) > maxPoints {
				coreData[i] = coreData[i][1:] // Remove oldest point
			}
		}
		if len(timestamps) > maxPoints {
			timestamps = timestamps[1:] // Sync timestamps with data
		}

		// Broadcast current metrics to all clients
		for client := range clients {
			err := client.WriteJSON(metrics)
			if err != nil {
				log.Printf("Error broadcasting to client: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}

func getDiskUsage(path string) (*disk.UsageStat, error) {
	usage, err := disk.Usage(path)
	if err != nil {
		return nil, err
	}
	return usage, nil
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	log.Printf("New WebSocket client connected")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket client disconnected: %v", err)
			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()
			break
		}
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func diskListHandler(w http.ResponseWriter, r *http.Request) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		http.Error(w, "Failed to get partitions", http.StatusInternalServerError)
		return
	}

	// Optionally build a custom list if you don't need all partition details.
	type diskInfo struct {
		Device     string   `json:"device"`
		Mountpoint string   `json:"mountpoint"`
		Fstype     string   `json:"fstype"`
		Opts       []string `json:"opts"`
	}
	disks := []diskInfo{}
	for _, p := range partitions {
		disks = append(disks, diskInfo{
			Device:     p.Device,
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Opts:       p.Opts,
		})
	}

	jsonData, err := json.Marshal(disks)
	if err != nil {
		http.Error(w, "Failed to marshal disks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func GetLocalIP() net.IP {
	conn, err := net.Dial("udp", "80.80.80.80:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddress := conn.LocalAddr().(*net.UDPAddr)

	return localAddress.IP
}

func GetLocalIPs() ([]net.IP, error) {
	var ips []net.IP
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addresses {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	return ips, nil
}

func constructDirString(basePath string) (string, error) {
	var availDirs string

	// Get list of year directories
	years, err := os.ReadDir(basePath)

	if err != nil {
		return "", err
	}
	var basePathstr = basePath + "|"
	for _, year := range years {

		if !year.IsDir() {
			continue // Skip if not a directory
		}
		yearPath := filepath.Join(basePath, year.Name())

		availDirs = basePathstr + "Y:" + year.Name()

		// Get months for this year
		months, err := os.ReadDir(yearPath)
		if err != nil {
			return "", err
		}

		for _, month := range months {

			if !month.IsDir() {
				continue
			}
			availDirs += "-M:" + month.Name() + "-D:"
			monthPath := filepath.Join(yearPath, month.Name())

			// Get days for this month
			days, err := os.ReadDir(monthPath)
			if err != nil {
				return "", err
			}

			for _, day := range days {
				if !day.IsDir() {
					continue
				}
				availDirs += day.Name() + ","
			}
		}
	}

	return availDirs, nil
}

// getMemoryStats returns memory usage statistics
func getMemoryStats() (used float64, free float64, total uint64, err error) {
	memory, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, err
	}

	// Convert values
	used = memory.UsedPercent            // Percentage of memory used
	free = 100.0 - memory.UsedPercent    // Percentage of memory free
	total = memory.Total / (1024 * 1024) // Total memory in MB

	return used, free, total, nil
}
