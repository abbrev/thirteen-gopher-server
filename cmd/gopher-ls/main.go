// Copyright 2025 Christopher Williams
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	docRoot    = os.Getenv("DOCUMENT_ROOT")
	serverName = os.Getenv("SERVER_NAME")
	serverPort = os.Getenv("SERVER_PORT")
	pwd        = getwd()
	gopherPath = pwd[len(docRoot):]
	sortBy     = os.Getenv("LS_SORTBY")
	hideExt    = os.Getenv("LS_HIDEEXT")
	mapExt     = os.Getenv("LS_MAPEXT")

	// "y" to enable
	sortRev        = getEnvBool("LS_SORTREV")
	includeHeader  = getEnvBool("LS_HEADER")
	includeParent  = getEnvBool("LS_PARENT")
	includeDetails = getEnvBool("LS_DETAILS")
)

var (
	hideExtensions = make(map[string]struct{})
	mapExtensions  = make(map[string]struct{})
)

func getEnvBool(name string) bool { return os.Getenv(name) == "y" }

func getwd() (cwd string) { cwd, _ = os.Getwd(); return }

func main() {
	// TODO extract hideExtensions and mapExtensions from hideExt and mapExt respectively
	//hideExtensions[".gph"] = struct{}{}
	//hideExtensions[".gmi"] = struct{}{}
	//mapExtensions[".gph"] = struct{}{}
	//mapExtensions[".gmi"] = struct{}{}

	fileInfos := getFileInfos(pwd)

	sortFunc := byName
	switch sortBy {
	case "n", "name":
		sortFunc = byName
	case "t", "time":
		sortFunc = byModTime
	case "s", "size":
		sortFunc = bySize
	}
	sort.Slice(fileInfos, reverser(sortFunc, fileInfos, sortRev))

	if includeHeader {
		writeInfoLine("[" + gopherPath + "/]")
		writeInfoLine("")
	}
	if includeParent && gopherPath != "" {
		lastSlashIx := strings.LastIndex(gopherPath, "/")
		parent := gopherPath[:lastSlashIx]
		writeDirEntry('1', "..", time.Time{}, 0, parent, false)
	}

	subs := []struct {
		from, to     string
		onlySelector bool
	}{
		{"%", "%25", false},
		{"?", "%3F", true},
		{"\t", "%09", false},
		{"\r", "%0D", false},
		{"\n", "%0A", false},
	}

	for i := range fileInfos {
		f := fileInfos[i]
		name := f.Name()
		size := f.Size()
		mtime := f.ModTime()
		selector := f.selector

		// URL escape special characters
		for _, sub := range subs {
			if !sub.onlySelector {
				name = strings.ReplaceAll(name, sub.from, sub.to)
			}
			selector = strings.ReplaceAll(selector, sub.from, sub.to)
		}
		gtype := '9'
		if f.IsDir() {
			gtype = '1'
		} else {
			ext := filepath.Ext(selector)
			if t, ok := fileTypes[ext]; ok {
				gtype = t
			} else if t, ok := fileTypes[filepath.Base(selector)]; ok {
				gtype = t
			}
			if ext == ".cgi" {
				size = -1 // don't report size for CGIs
			}
		}
		writeDirEntry(gtype, name, mtime, size, selector, includeDetails)
	}
}

// shamelessly lifted from geomyidae source (with default and nonsensical types removed)
var fileTypes = map[string]rune{
	".gmi":      '1',
	".gph":      '1',
	".cgi":      '0',
	".gif":      'g',
	".jpg":      'I',
	".png":      'I',
	".bmp":      'I',
	".txt":      '0',
	".vtt":      '0',
	".html":     'h',
	".htm":      'h',
	".xhtml":    '0',
	".css":      '0',
	".md":       '0',
	".asc":      '0',
	".adoc":     '0',
	".c":        '0',
	".sh":       '0',
	".patch":    '0',
	"gophermap": '0',
	".ogg":      's',
	".opus":     's',
	".wav":      's',
	".mp3":      's',
	".pdf":      'p',
}

func writeInfoLine(text string) {
	writeMenuLine('i', text, "", serverName, serverPort)
}

func writeErrorLine(text string) {
	writeMenuLine('3', text, "", serverName, serverPort)
}

func writeDirEntry(gtype rune, name string, mtime time.Time, size int64, path string, includeDetails bool) {
	nameStr := ""
	minLength := 41
	maxLength := 41

	if gtype == '1' {
		includeDetails = false
	}
	if !includeDetails {
		maxLength = 69
		minLength = 0
	}

	if len(name) > maxLength {
		name = name[:maxLength-3] + "..."
	}

	details := ""
	if includeDetails {
		mtimeStr := mtime.Format("2006-01-02 15:04")
		sizeStr := humanSize(size)
		details = fmt.Sprintf("  %16.16s  %8.8s", mtimeStr, sizeStr)
	}
	nameStr = fmt.Sprintf("%-*.*s%s", minLength, maxLength, name, details)
	writeMenuLine(gtype, nameStr, path, serverName, serverPort)
}

func writeMenuLine(gtype rune, text, path, serverName, serverPort string) {
	fmt.Printf("%c%s\t%s\t%s\t%s\r\n", gtype, text, path, serverName, serverPort)
}

func reverser(lessfn func([]gopherFileInfo) func(int, int) bool, fileInfos []gopherFileInfo, rev bool) func(int, int) bool {
	less := lessfn(fileInfos)
	return func(i, j int) bool {
		return rev != less(i, j)
	}
}

func byName(fileInfos []gopherFileInfo) func(i, j int) bool {
	return func(i, j int) bool {
		aIsDir, bIsDir := fileInfos[i].IsDir(), fileInfos[j].IsDir()
		// if only one is a directory, the directory comes first
		if aIsDir != bIsDir {
			return aIsDir
		}
		return fileInfos[i].Name() < fileInfos[j].Name()
	}
}

func byModTime(fileInfos []gopherFileInfo) func(i, j int) bool {
	return func(i, j int) bool {
		aIsDir, bIsDir := fileInfos[i].IsDir(), fileInfos[j].IsDir()
		// if only one is a directory, the directory comes first
		if aIsDir != bIsDir {
			return aIsDir
		}
		// always sort directories first and by name
		if aIsDir && bIsDir {
			return fileInfos[i].Name() < fileInfos[j].Name()
		}

		a, b := fileInfos[i].ModTime(), fileInfos[j].ModTime()
		if a.Equal(b) {
			return fileInfos[i].Name() < fileInfos[j].Name()
		}
		return a.Before(b)
	}
}

func bySize(fileInfos []gopherFileInfo) func(i, j int) bool {
	return func(i, j int) bool {
		aIsDir, bIsDir := fileInfos[i].IsDir(), fileInfos[j].IsDir()
		// if only one is a directory, the directory comes first
		if aIsDir != bIsDir {
			return aIsDir
		}
		// always sort directories first and by name
		if aIsDir && bIsDir {
			return fileInfos[i].Name() < fileInfos[j].Name()
		}

		a, b := fileInfos[i].Size(), fileInfos[j].Size()
		if a == b {
			return fileInfos[i].Name() < fileInfos[j].Name()
		}
		return a < b
	}
}

func humanSize(size int64) string {
	if size < 0 {
		return ""
	}
	if size < 1000 {
		return fmt.Sprintf("%d  B", size)
	}
	prefix := ' '
	for _, p := range []struct {
		prefix     rune
		multiplier int64
	}{
		{'E', int64(1e18)},
		{'P', int64(1e15)},
		{'T', int64(1e12)},
		{'G', int64(1e9)},
		{'M', int64(1e6)},
		{'k', int64(1e3)},
	} {
		m10 := p.multiplier / 10
		if size > (p.multiplier - m10) {
			prefix = p.prefix
			// round up as `ls --si` does
			size = (size + m10 - 1) / m10
			break
		}
	}
	return fmt.Sprintf("%d.%d %cB", size/10, size%10, prefix)
}

type gopherFileInfo struct {
	os.FileInfo

	name     string
	selector string
	isMap    bool
}

// return the link name rather than the target name, in the case of symlinks
func (g *gopherFileInfo) Name() string { return g.name }

func (g *gopherFileInfo) IsDir() bool { return g.isMap }

// Get a list of files in current directory.
// Evaluate symlinks and filter out unreachable files.
func getFileInfos(dir string) []gopherFileInfo {
	fileInfos, _ := ioutil.ReadDir(dir)
	gopherFileInfos := make([]gopherFileInfo, 0, len(fileInfos))

	for i := range fileInfos {
		f := fileInfos[i]
		name := f.Name()
		if name == "index.map" || name == "index.cgi" || name[0] == '.' {
			continue
		}
		target, err := filepath.EvalSymlinks(pwd + "/" + name)
		if err != nil {
			continue
		}
		//writeInfoLine("target: " + target)
		if target != docRoot && !strings.HasPrefix(target, docRoot+"/") {
			continue
		}
		f, err = os.Stat(target)
		if err != nil || f.Mode()&0004 == 0 {
			continue
		}
		selector := target[len(docRoot):]
		isMap := f.IsDir()

		ext := filepath.Ext(name)
		if _, ok := hideExtensions[ext]; ok && name != ext {
			// chop off extension from name
			name = name[:len(name)-len(ext)]
		}
		selectorExt := filepath.Ext(selector)
		if _, ok := mapExtensions[selectorExt]; ok {
			isMap = true
		}
		if _, ok := hideExtensions[selectorExt]; ok && selector != selectorExt {
			// chop off extension from selector
			selector = selector[:len(selector)-len(ext)]
		}

		gopherFileInfos = append(gopherFileInfos, gopherFileInfo{f, name, selector, isMap})
	}

	return gopherFileInfos
}
