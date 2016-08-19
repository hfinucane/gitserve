package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
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

func git_list(hash string) ([]byte, error) {
	objects, err := lstree(hash)
	if err != nil {
		return nil, err
	}
	fmt.Println("processing ", len(objects), "@", hash)
	t, err := template.New("list").Parse(`<html>
	<body>
	<ul>
	{{- range .}}
	<li><a href="{{.Name}}">{{.Name}}</a>
	{{- end}}
	</ul>
	</body>
	</html>`)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, objects)
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

func get_object(starting_hash, final_path string) ([]byte, error) {
	fmt.Println("starting hash @get_object", starting_hash, " @final_path: ", final_path)
	objects, err := lstree(starting_hash)
	if err != nil {
		return nil, err
	}

	next_prefix, rest := PathRsplit(final_path)
	for _, object := range objects {
		next_next_prefix, _ := PathRsplit(rest)
		fmt.Println("inner loop ", next_prefix, "rest_next", next_next_prefix, "curobname", object.Name)
		if object.Name == next_next_prefix {
			if object.ObjectType == GitTree {
				return get_object(object.Hash, rest)
			} else {
				// XXX let's do a better job here
				return nil, errors.New(fmt.Sprintf("Unsupported object type, ", object.ObjectType))
			}
		} else if object.Name == next_prefix {
			if object.ObjectType == GitBlob {
				return git_show(object.Hash)
			} else if object.ObjectType == GitTree {
				return git_list(object.Hash)
			}
		}
	}
	return nil, errors.New("file not found in tree")
}

type Lengthwise []string

func (s Lengthwise) Len() int {
	return len(s)
}
func (s Lengthwise) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s Lengthwise) Less(i, j int) bool {
	var i_slash_count, j_slash_count int
	for _, c := range s[i] {
		if c == '/' {
			i_slash_count++
		}
	}
	for _, c := range s[j] {
		if c == '/' {
			j_slash_count++
		}
	}
	if i_slash_count != j_slash_count {
		return i_slash_count > j_slash_count
	}
	return len(s[i]) > len(s[j])
}

func servePath(writer http.ResponseWriter, request *http.Request) {
	refs, err := get_refs()
	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	sort.Sort(Lengthwise(refs))

	fmt.Println("all refs: ", refs)

	// Make sure we're in the right place doing the right thing
	path_components := strings.Split(request.URL.Path, "/")
	fmt.Println("Path components", path_components)
	if path_components[0] != "" || path_components[1] != "blob" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	var target_name = path_components[2] // IE, /blob/path_components[2]/
	var rest = strings.Join(path_components[3:], "/")
	// Check if there is a human-readable ref, which may contain slashes, here
	for _, ref := range refs {
		fmt.Println("Checking ", ref, " against ", request.URL.Path)
		if strings.HasPrefix(request.URL.Path[len("/blob/"):], ref+"/") {
			// Taking advantage of the fact that `refs` is sorted,
			// and that git does not allow both foo/bar and /foo to be refs
			rest = request.URL.Path[len("/blob/")+len(ref)+1:]
			fmt.Println("Rest matched to: ", rest)
			break
		}
	}

	blob, err := get_object(target_name, rest)

	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, err)
		return
	}
	fmt.Fprint(writer, string(blob))
}

func main() {
	var git_path *string = flag.String("repo", ".", "git repo to serve")
	var address *string = flag.String("listen", "0.0.0.0", "what address to listen to")
	var port *int = flag.Int("port", 6504, "port to listen on")
	flag.Parse()

	// Move to the git repo
	err := os.Chdir(*git_path)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	// Verify this is is a git directory
	cmd := exec.Command("git", "status")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Git didn't like ", *git_path, " got ", err)
		os.Exit(3)
	}

	http.HandleFunc("/blob/", servePath)
	http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil)
}
