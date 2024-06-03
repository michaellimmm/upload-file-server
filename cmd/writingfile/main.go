package main

import (
	"bytes"
	"io"
	"os"
)

func main() {
	// write file
	d1 := []byte("hello\ngo\n")
	err := os.WriteFile("test1.txt", d1, 0644)
	check(err)

	f, err := os.Create("test.txt")
	check(err)
	defer f.Close()

	fileData := &bytes.Buffer{}
	fileData.WriteString("hello world")

	_, err = io.Copy(f, fileData)
	check(err)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
