# EUMETCAST File Manager User Guide

## Overview

EUMETCAST File Manager is a web application that organizes files received from EUMETCAST into date-based directories, manages disk space by automatically removing older data, and provides a monitoring interface for system resources.

## Getting Started

Install the web app as a deamon in Linux or a service in Windows. You can run also the program in a console.

1. Access the web interface by opening your browser and navigating to:
   ```
   http://localhost:7000
   ```

2. The port number can be modified in the configuration file.

## Main Features

### File Organization

Files received from EUMETCAST are automatically sorted into directories with the format `YYYY/MM/DD` based on date information extracted from the filenames. The date in the filenames can be of the form YYYYMMDD or YYYYDDD. The system uses the configuration file to determine:

- Which files to process (based on filename patterns)
- Where to extract the date information from each filename
- The date format used in the filename

### Disk Space Management

The system automatically monitors available disk space and removes the oldest data directories when free space falls below configured thresholds:

- For `/media/hugo/Vol4T`: Maintains at least 30% of free space
- For `/media/hugo/Vol3T`: Maintains at least 20% of free space

### Web Interface

The interface provides real-time monitoring of:

1. **CPU Activity**: Current CPU usage statistics
2. **Disk Space**: Available and used space for each configured disk
3. **Directory Listing**: Shows the available date-based directories for each base path

## Configuration

The system is configured using a YAML file with the following key sections:

### File Templates

Define patterns for incoming files and how to extract date information:

```yaml
filetemplates:
  - filetemplate: "avhrr_*_noaa19.hrp.bz2"
    startdate: 6
    datelayout: YYYYMMDD
```

Each template specifies:
- `filetemplate`: Pattern to match filenames
- `startdate`: Character position where the date information begins
- `datelayout`: Format of the date in the filename (YYYYMMDD or YYYYDDD)

### Base Paths

Directories where incoming files are stored and managed:

```yaml
basepaths:
  - /media/hugo/Vol4T/received/hvs-1/E1H-RDS-1
  - /media/hugo/Vol4T/received/bas/E1B-TPG-1
```

### Disk Space Management

Configure thresholds for available disk space:

```yaml
disks:
  - diskname: "/media/hugo/Vol4T"
    freediskspace: 30
```

### Server Configuration

```yaml
portnumber: 7000
```
![CleanUp](https://github.com/user-attachments/assets/b29d2309-519d-45b2-a383-1fd2e3b99d19)


