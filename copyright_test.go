// Copyright (C) 2025 Kent Behrends
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyrightHeader is the exact first line every .go source file must carry.
const copyrightHeader = "// Copyright (C) 2025 Kent Behrends"

// TestCopyrightHeaders walks every .go file in the module tree and asserts
// that the first non-empty line is the GPL v3 copyright notice.
// This test acts as a lightweight CI gate to prevent new files being added
// without the required license header.
func TestCopyrightHeaders(t *testing.T) {
	t.Helper()

	root := "." // run from module root via `go test ./...`

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, hidden dirs, and test-build artefacts
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || (len(name) > 0 && name[0] == '.') || name == "bin" || name == "coverage" {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			t.Errorf("cannot open %s: %v", path, err)
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		firstLine := ""
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), " \t\r")
			if line != "" {
				firstLine = line
				break
			}
		}
		if scanner.Err() != nil {
			t.Errorf("error reading %s: %v", path, scanner.Err())
			return nil
		}

		if firstLine != copyrightHeader {
			t.Errorf("missing GPL copyright header in %s\n  want: %q\n   got: %q",
				path, copyrightHeader, firstLine)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}
}

// TestTemplatesNoInlineCSS verifies that no Go source file contains an
// inline <style> block (all CSS must be served from static/).
func TestTemplatesNoInlineCSS(t *testing.T) {
	t.Helper()
	// needle matches an opening <style tag inside a Go string literal
	checkNoInlineAsset(t, "<style", "inline CSS style block (move CSS to static/css/)")
}

// TestTemplatesNoInlineJS verifies that no Go source file contains an
// inline script block without a src= attribute (all JS logic must be
// served from static/ — script src references are allowed).
func TestTemplatesNoInlineJS(t *testing.T) {
	t.Helper()
	// needle matches a bare opening script tag with no src attribute
	checkNoInlineAsset(t, "<script>", "inline JS script block without src (move JS to static/js/)")
}

// checkNoInlineAsset walks .go files and fails the test if any line contains needle.
func checkNoInlineAsset(t *testing.T, needle, description string) {
	t.Helper()
	root := "."
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || (len(name) > 0 && name[0] == '.') || name == "bin" || name == "coverage" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			t.Errorf("cannot open %s: %v", path, err)
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), needle) {
				t.Errorf("%s:%d contains %s", path, lineNum, description)
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}
}
