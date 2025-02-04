// Copyright 2025 Christopher Williams
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitRequest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		request  []byte
		selector string
		path     string
		query    string
		search   string
	}{
		{
			"Empty",
			[]byte(""),
			"", "", "", "",
		},
		{
			"Root",
			[]byte("/"),
			"/", "/", "", "",
		},
		{
			"Query",
			[]byte("/script?query"),
			"/script?query", "/script", "query", "",
		},
		{
			"Query and search",
			[]byte("/script?query\tsearch"),
			"/script?query", "/script", "query", "search",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := assert.New(t)
			selector, path, query, search := splitRequest(tc.request)
			at.Equal(tc.selector, selector)
			at.Equal(tc.path, path)
			at.Equal(tc.query, query)
			at.Equal(tc.search, search)
		})
	}
}

func TestSplitPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		fsPath   string
		scriptPath string
		pathInfo string
		isError  bool
	}{
		{
			"Empty",
			"",
			"tests/index.map", "", "", false,
		},
		{
			"Root",
			"/",
			"tests/index.map", "", "/", false,
		},
		{
			"Text file",
			"/text.txt",
			"tests/text.txt", "/text.txt", "", false,
		},
		{
			"Percent encoding",
			"%2Ftext.txt",
			"tests/text.txt", "/text.txt", "", false,
		},
		{
			"Invalid percent encoding",
			"%2.text.txt",
			"", "", "", true,
		},
		{
			"Contiguous slashes",
			"///foo//text.txt",
			"tests/foo/text.txt", "/foo/text.txt", "", false,
		},
		{
			"Trailing slash",
			"/foo/text.txt/",
			"tests/foo/text.txt", "/foo/text.txt", "/", false,
		},
		{
			"Contiguous and trailing slashes",
			"///foo//text.txt/",
			"tests/foo/text.txt", "/foo/text.txt", "/", false,
		},
		{
			"CGI",
			"/foo/bar/cgi.cgi",
			"tests/foo/bar/cgi.cgi", "/foo/bar/cgi.cgi", "", false,
		},
		{
			"CGI with path info",
			"/foo/bar/cgi.cgi/path/info",
			"tests/foo/bar/cgi.cgi", "/foo/bar/cgi.cgi", "/path/info", false,
		},
		{
			"Path info",
			"/foo/bar/path/info",
			"tests/foo/bar/index.cgi", "/foo/bar", "/path/info", false,
		},
		{
			"Dot",
			"/foo/./bar",
			"tests/foo/bar/index.cgi", "/foo/bar", "", false,
		},
		{
			"Dot at the end",
			"/foo/text.txt/bar/.",
			"tests/foo/text.txt", "/foo/text.txt", "/bar", false,
		},
		{
			"Dot in PATH_INFO",
			"/foo/bar/cgi.cgi/./bar",
			"tests/foo/bar/cgi.cgi", "/foo/bar/cgi.cgi", "/bar", false,
		},
		{
			"Dot at the end in PATH_INFO",
			"/foo/bar/cgi.cgi/bar/.",
			"tests/foo/bar/cgi.cgi", "/foo/bar/cgi.cgi", "/bar", false,
		},
		{
			"Dot dot",
			"/foo/../bar",
			"tests/index.map", "", "/bar", false,
		},
		{
			"Dot dot at the end 2",
			"/foo/bar/..",
			"tests/index.map", "", "/foo", false,
		},
		{
			"Dot dot in PATH_INFO",
			"/foo/bar/cgi.cgi/../bar",
			"tests/foo/bar/index.cgi", "/foo/bar", "/bar", false,
		},
		{
			"Dot dot at the end in PATH_INFO",
			"/foo/bar/cgi.cgi/bar/..",
			"tests/foo/bar/cgi.cgi", "/foo/bar/cgi.cgi", "", false,
		},
		{
			"No leading slash",
			"foo",
			"tests/index.map", "", "/foo", false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := assert.New(t)
			fsPath, scriptPath, pathInfo, err := splitPath("tests", tc.path)
			at.Equal(tc.isError, err != nil)
			if err == nil {
				at.Equal(tc.fsPath, fsPath)
				at.Equal(tc.scriptPath, scriptPath)
				at.Equal(tc.pathInfo, pathInfo)
			}
		})
	}
}
