package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type GitObjectType string

const (
	GitBlob   = "blob"
	GitTree   = "tree"
	GitCommit = "commit"
	GitTag    = "tag"
)

type GitObject struct {
	Permission uint32
	ObjectType GitObjectType
	Hash       string
	Name       string
}

func PathRsplit(path string) (first, rest string) {
	if path == "" || path == "/" {
		return "", ""
	}
	i := strings.Index(path, "/")
	if i == 0 {
		return PathRsplit(path[1:]) // Just... forget handling this
	} else if i == -1 {
		return path, ""
	}
	return path[:i], path[i+1:]
}

func git_show(hash string) ([]byte, error) {
	cmd := exec.Command("git", "show", hash)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("could not attach a pipe to the command ", err)
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		fmt.Println("command failed with ", err)
		return nil, err
	}
	return ioutil.ReadAll(stdout)
}

func get_refs() ([]string, error) {
	var refs []string
	refs_cmd := exec.Command("git", "show-ref")
	stdout, err := refs_cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	refs_cmd.Start()
	// Should consider hoisting this
	refs_r, err := regexp.Compile("^([0-9a-f]{40})\\s+(.+)")
	if err != nil {
		return nil, err
	}
	buf_stdout := bufio.NewReader(stdout)
	for line, err := buf_stdout.ReadBytes('\n'); err == nil; line, err = buf_stdout.ReadBytes('\n') {
		results := refs_r.FindSubmatch(line)
		if len(results) != 3 {
			return nil, errors.New(fmt.Sprintf("Confused by your refs- got %d matches out of something that's supposed to have 2 fields. Line: %s Results: %q", len(results), line, results))
		}
		refs = append(refs, string(results[2][len("refs/"):]))
	}
	return refs, err
}

func git_list(hash, starting_string string) ([]byte, error) {
	objects, err := lstree(hash)
	if err != nil {
		return nil, err
	}
	fmt.Println("processing ", len(objects), "@", hash)
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
	}{objects, starting_string}
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
		fmt.Println("could not attach a pipe to the command ", err)
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		fmt.Println("command failed with ", err)
		return nil, err
	}
	buf_stdout := bufio.NewReader(stdout)

	var obs []GitObject
	// for reasons I do not understand, saying "this ends with a newline" means
	// the newline gets eaten by the final capture. Also, ending with a '$' breaks
	// the whole match.
	tree_r, err := regexp.Compile("^([0-9]+)\\s([a-z]+)\\s([a-z0-9]+)\\s(.+)")
	if err != nil {
		panic(err)
	}
	for line, err := buf_stdout.ReadBytes('\n'); err == nil; line, err = buf_stdout.ReadBytes('\n') {
		results := tree_r.FindSubmatch(line)
		if len(results) != 5 {
			// We expect to get back [original, perms, ob, hash, filename]
			return nil, errors.New(fmt.Sprintf("Unexpected parse of `git ls-tree` output: %q", results))
		}
		permissions, err := strconv.ParseUint(string(results[1]), 10, 32)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse permissions for file %s, got %s", results[4], results[1]))
		}
		obs = append(obs, GitObject{uint32(permissions), GitObjectType(results[2]), string(results[3]), string(results[4])})
	}
	return obs, err
}

func get_object(starting_hash, starting_string, final_path string) ([]byte, error) {
	fmt.Println("starting hash @get_object", starting_hash, " @final_path: ", final_path)
	objects, err := lstree(starting_hash)
	if err != nil {
		return nil, err
	}

	if final_path == "" {
		return git_list(starting_hash, starting_string)
	}

	next_prefix, rest := PathRsplit(final_path)
	fmt.Println("next_prefix ", next_prefix, " rest ", rest)

	next_next_prefix, _ := PathRsplit(rest)
	for _, object := range objects {
		fmt.Println("inner loop next_next_prefix", next_next_prefix, "curobname", object.Name)
		if rest == "" { // end of the line
			if object.Name == next_prefix {
				if object.ObjectType == GitTree {
					return git_list(object.Hash, starting_string)
				} else if object.ObjectType == GitBlob {
					return git_show(object.Hash)
				} else {
					// XXX let's do a better job here
					return nil, errors.New(fmt.Sprintf("Unsupported object type, ", object.ObjectType))
				}
			}
		} else {
			if object.Name == next_prefix {
				if object.ObjectType == GitTree {
					return get_object(object.Hash, starting_string, rest)
				} else if object.ObjectType == GitBlob {
					return nil, errors.New(fmt.Sprintf("This is a directory, not an object, ", object.ObjectType, object.Hash, object.Name))
				} else {
					// XXX let's do a better job here
					return nil, errors.New(fmt.Sprintf("Unsupported object type, ", object.ObjectType))
				}
			}
		}
	}
	fmt.Printf("filenotfound, next_prefix %q rest %q\n", next_prefix, rest)
	return nil, errors.New("file not found in tree")
}
