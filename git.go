package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GitObjectType Represents the type of a git object. Content is in "blob"
// types, everything else is metadata.
type GitObjectType string

const (
	// GitBlob represents content
	GitBlob = "blob"
	// GitTree represents a 'filesystem' in Git
	GitTree = "tree"
	// GitCommit represents a snapshot of state with comment
	GitCommit = "commit"
	// GitTag represents a named snapshot of state with comment
	GitTag = "tag"
)

// GitObject is a metadata blob describing everything about an object in git
// but the content
type GitObject struct {
	Permission uint32
	ObjectType GitObjectType
	Hash       string
	Name       string
}

func pathRsplit(path string) (first, rest string) {
	if path == "" || path == "/" {
		return "", ""
	}
	i := strings.Index(path, "/")
	if i == 0 {
		return pathRsplit(path[1:]) // Just... forget handling this
	} else if i == -1 {
		return path, ""
	}
	return path[:i], path[i+1:]
}

func gitShow(hash string) ([]byte, error) {
	cmd := exec.Command("git", "show", hash)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(stdout)
}

func getRefs() ([]string, error) {
	var refs []string
	refsCmd := exec.Command("git", "show-ref")
	stdout, err := refsCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	refsCmd.Start()
	// Should consider hoisting this
	refsR, err := regexp.Compile("^([0-9a-f]{40})\\s+(.+)")
	if err != nil {
		return nil, err
	}
	bufStdout := bufio.NewReader(stdout)
	for line, err := bufStdout.ReadBytes('\n'); err == nil; line, err = bufStdout.ReadBytes('\n') {
		results := refsR.FindSubmatch(line)
		if len(results) != 3 {
			return nil, fmt.Errorf("Confused by your refs- got %d matches out of something that's supposed to have 2 fields. Line: %s Results: %q", len(results), line, results)
		}
		refs = append(refs, string(results[2][len("refs/"):]))
	}
	return refs, err
}

func gitList(hash, startingString string) ([]byte, error) {
	objects, err := lstree(hash)
	if err != nil {
		return nil, err
	}
	t, err := template.New("list").Parse(`<html>
	<body>
	<ul>
	{{ $prefix := .Prefix }}
	{{- range $element := .Objects}}
	<li><a href="{{$prefix}}/{{$element.Name}}{{ if eq $element.ObjectType "tree"}}/{{ end }}">{{$element.Name}}</a>
	{{- end}}
	</ul>
	</body>
	</html>`)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	var data = struct {
		Objects []GitObject
		Prefix  string
	}{objects, startingString}
	err = t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

func lstree(commit string) ([]GitObject, error) {
	// Inspect the cwd
	cmd := exec.Command("git", "ls-tree", commit)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	bufStdout := bufio.NewReader(stdout)

	var obs []GitObject
	// for reasons I do not understand, saying "this ends with a newline" means
	// the newline gets eaten by the final capture. Also, ending with a '$' breaks
	// the whole match.
	treeR, err := regexp.Compile("^([0-9]+)\\s([a-z]+)\\s([a-z0-9]+)\\s(.+)")
	if err != nil {
		panic(err)
	}
	for line, err := bufStdout.ReadBytes('\n'); err == nil; line, err = bufStdout.ReadBytes('\n') {
		results := treeR.FindSubmatch(line)
		if len(results) != 5 {
			// We expect to get back [original, perms, ob, hash, filename]
			return nil, fmt.Errorf("Unexpected parse of `git ls-tree` output: %q", results)
		}
		permissions, err := strconv.ParseUint(string(results[1]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Could not parse permissions for file %s, got %s", results[4], results[1])
		}
		obs = append(obs, GitObject{uint32(permissions), GitObjectType(results[2]), string(results[3]), string(results[4])})
	}
	return obs, err
}

func getObject(startingHash, startingString, finalPath string) ([]byte, error) {
	objects, err := lstree(startingHash)
	if err != nil {
		return nil, err
	}

	if finalPath == "" {
		return gitList(startingHash, startingString)
	}

	nextPrefix, rest := pathRsplit(finalPath)

	for _, object := range objects {
		if rest == "" { // end of the line
			if object.Name == nextPrefix {
				if object.ObjectType == GitTree {
					return gitList(object.Hash, startingString)
				} else if object.ObjectType == GitBlob {
					return gitShow(object.Hash)
				} else {
					return nil, fmt.Errorf("Unsupported object type, %s", object.ObjectType)
				}
			}
		} else {
			if object.Name == nextPrefix {
				if object.ObjectType == GitTree {
					return getObject(object.Hash, startingString, rest)
				} else if object.ObjectType == GitBlob {
					return nil, fmt.Errorf("This is a directory, not an object, %s %s %s", object.ObjectType, object.Hash, object.Name)
				} else {
					return nil, fmt.Errorf("Unsupported object type, %s", object.ObjectType)
				}
			}
		}
	}
	return nil, fmt.Errorf("file not found in tree %s %s", startingString, finalPath)
}
