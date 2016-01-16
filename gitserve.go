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

func servePath(writer http.ResponseWriter, request *http.Request) {
	path_components := strings.Split(request.URL.Path, "/")
	if path_components[0] != "" || path_components[1] != "blob" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	blob, err := get_object(path_components[2], strings.Join(path_components[3:], "/"))
	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprint(writer, string(blob))
}

func main() {
	var git_path *string = flag.String("repo", ".", "git repo to serve")
	var internal_path *string = flag.String("path", ".", "path to the object")
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

	blob, err := get_object("HEAD", *internal_path)
	fmt.Println(string(blob))
	fmt.Println(err)
}
