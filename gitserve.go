package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
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
	fmt.Println("starting hash @get_object", starting_hash)
	objects, err := lstree(starting_hash)
	if err != nil {
		return nil, err
	}

	next_prefix, rest := path.Split(final_path)
	for _, object := range objects {
		fmt.Println(next_prefix, object.Name)
		if object.Name == next_prefix {
			if object.ObjectType == GitTree {
				return get_object(object.Hash, rest)
			} else {
				// XXX let's do a better job here
				return nil, errors.New("Unsupported object type")
			}
		} else if object.Name == rest {
			if object.ObjectType == GitBlob {
				return git_show(object.Hash)
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
	return len(s[i]) < len(s[j])
}

func servePath(writer http.ResponseWriter, request *http.Request) {
	refs, err := get_refs()
	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	sort.Sort(Lengthwise(refs))

	// Make sure we're in the right place doing the right thing
	path_components := strings.Split(request.URL.Path, "/")
	if path_components[0] != "" || path_components[1] != "blob" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	var target_name = path_components[2] // IE, /blob/path_components[2]/
	// Check if there is a human-readable ref, which may contain slashes, here
	for _, ref := range refs {
		if strings.HasPrefix(request.URL.Path[len("/blob/"):], ref+"/") {
			target_name = ref
			// Taking advantage of the fact that `refs` is sorted,
			// and that git does not allow both foo/bar and /foo to be refs
			break
		}
	}

	blob, err := get_object(target_name, strings.Join(path_components[3:], "/"))

	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
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
