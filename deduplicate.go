package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

var (
	diff       bool
	hash       bool
	duplicates bool
)

func main() {
	var (
		a string
		b string
	)

	flag.StringVar(&a, "a", "", "first directory")
	flag.StringVar(&b, "b", "", "second directory")
	flag.BoolVar(&diff, "diff", false, "find differences based on names")
	flag.BoolVar(&hash, "hash", false, "check file hashes")
	flag.BoolVar(&duplicates, "dup", false, "check for duplicated files")
	flag.Parse()

	if duplicates {
		hash = true
	}

	fmt.Printf("list files in %s and %s\n", a, b)
	aFiles, err := listFiles(a)
	if err != nil {
		log.Fatal(err)
	}

	bFiles, err := listFiles(b)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("compare files")
	aOnly, bOnly := Diff(aFiles, bFiles)

	if diff {
		color.Cyan("only in A:")
		color.Cyan(aOnly.String())
		fmt.Println()
		color.Yellow("only in B:")
		color.Yellow(bOnly.String())
	}

	if duplicates {
		aFiles.Duplicates()
		bFiles.Duplicates()
	}
}

func listFiles(root string) (FileList, error) {
	list := FileList{}

	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		path = strings.TrimPrefix(path, root)
		list[path] = nil

		return nil
	})
	if err != nil {
		return nil, err
	}

	var (
		wg   sync.WaitGroup
		lock sync.Mutex
	)

	bar := progressbar.New(len(list))

	list2 := FileList{}
	for path := range list {
		wg.Add(1)

		go func(path string) {
			lock.Lock()
			defer lock.Unlock()

			hash := hashFile(root + path)

			path = strings.ToLower(path)
			_, ok := list2[path]
			if ok {
				fmt.Println("path collision:", path)
			}

			list2[path] = hash

			bar.Add(1)
			wg.Done()
		}(path)
	}

	wg.Wait()

	fmt.Println()

	return list2, nil
}

type FileList map[string][]byte

func (f FileList) String() string {
	var s []string

	for file := range f {
		s = append(s, file)
	}

	sort.Strings(s)

	return strings.Join(s, "\n")
}

func (f FileList) Duplicates() {
	hashes := map[string][]string{}

	for file, hash := range f {
		h := fmt.Sprintf("%x", hash)
		hashes[h] = append(hashes[h], file)
	}

	for _, files := range hashes {
		if len(files) == 1 {
			continue
		}

		fmt.Println("duplicate files:", files)
	}
}

func Diff(a, b FileList) (aOnly, bOnly FileList) {
	aOnly = FileList{}
	bOnly = FileList{}

	for f, h := range a {
		if b[f] == nil {
			aOnly[f] = h
			continue
		}

		if !bytes.Equal(a[f], b[f]) {
			fmt.Printf("error in file %s (%x, %x)\n", f, a[f], b[f])
		}
	}

	for f, h := range b {
		if a[f] == nil {
			bOnly[f] = h
		}
	}

	return aOnly, bOnly
}

func hashFile(path string) []byte {
	if !hash {
		return []byte("no hash computed")
	}

	hash := sha256.New()

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	_, err = io.Copy(hash, f)
	if err != nil {
		log.Fatal(err)
	}

	return hash.Sum([]byte{})
}
