// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package unpackit

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/bradfitz/iter"
	"github.com/hooklift/assert"
)

func TestUnpack(t *testing.T) {
	var tests = []struct {
		filepath string
		files    int
	}{
		{"./fixtures/test.tar.bzip2", 2},
		{"./fixtures/test.tar.gz", 2},
		{"./fixtures/test.tar.xz", 2},
		{"./fixtures/test.zip", 2},
		{"./fixtures/filetest.zip", 3},
		{"./fixtures/test.tar", 2},
		{"./fixtures/cfgdrv.iso", 1},
		{"./fixtures/test2.tar.gz", 4},
		{"./fixtures/tar-without-directory-entries.tar.gz", 1},
		{"./fixtures/testfolder.zip", 3},
	}

	for _, test := range tests {
		t.Run(test.filepath, func(t *testing.T) {
			tempDir, err := ioutil.TempDir(os.TempDir(), "unpackit-tests-"+path.Base(test.filepath)+"-")
			assert.Ok(t, err)
			defer os.RemoveAll(tempDir)

			file, err := os.Open(test.filepath)
			assert.Ok(t, err)
			defer file.Close()

			destPath, err := Unpack(file, tempDir)
			assert.Ok(t, err)

			length := calcNumberOfFiles(t, destPath)
			assert.Equals(t, test.files, length)
		})
	}
}

func TestMagicNumber(t *testing.T) {
	var tests = []struct {
		filepath string
		offset   int
		ftype    string
	}{
		{"./fixtures/test.tar.bzip2", 0, "bzip"},
		{"./fixtures/test.tar.gz", 0, "gzip"},
		{"./fixtures/test.tar.xz", 0, "xz"},
		{"./fixtures/test.zip", 0, "zip"},
		{"./fixtures/test.tar", 257, "tar"},
	}

	for _, test := range tests {
		file, err := os.Open(test.filepath)
		assert.Ok(t, err)

		ftype, err := magicNumber(bufio.NewReader(file), test.offset)
		file.Close()
		assert.Ok(t, err)

		assert.Equals(t, test.ftype, ftype)
	}
}

func TestUntar(t *testing.T) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Add some files to the archive.
	var files = []struct {
		Name, Body string
	}{
		{"readme.txt", "This archive contains some text files."},
		{"gopher.txt", "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
		{"todo.txt", "Get animal handling licence."},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Size: int64(len(file.Body)),
		}
		err := tw.WriteHeader(hdr)
		assert.Ok(t, err)

		_, err = tw.Write([]byte(file.Body))
		assert.Ok(t, err)
	}

	// Make sure to check the error on Close.
	err := tw.Close()
	assert.Ok(t, err)

	// Open the tar archive for reading.
	r := bytes.NewReader(buf.Bytes())
	destDir, err := ioutil.TempDir(os.TempDir(), "terraform-vix")
	assert.Ok(t, err)
	defer os.RemoveAll(destDir)

	_, err = Untar(r, destDir)
	assert.Ok(t, err)
}

func TestUntarOpenFileResourceLeak(t *testing.T) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Add some files to the archive.
	var files = make([]struct {
		Name, Body string
	}, 2000)

	for fileNum := range iter.N(2000) {
		files[fileNum] = struct {
			Name, Body string
		}{
			Name: fmt.Sprintf("file-%d.txt", fileNum),
			Body: fmt.Sprintf("This is file number %d\n", fileNum),
		}
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Size: int64(len(file.Body)),
		}
		err := tw.WriteHeader(hdr)
		assert.Ok(t, err)

		_, err = tw.Write([]byte(file.Body))
		assert.Ok(t, err)
	}

	// Make sure to check the error on Close.
	err := tw.Close()
	assert.Ok(t, err)

	// Open the tar archive for reading.
	r := bytes.NewReader(buf.Bytes())
	destDir, err := ioutil.TempDir(os.TempDir(), "terraform-vix")
	assert.Ok(t, err)
	defer os.RemoveAll(destDir)

	_, err = Untar(r, destDir)
	assert.Ok(t, err)
}

func TestUnzipOpenFileResourceLeak(t *testing.T) {
	tempPath, err := ioutil.TempDir(os.TempDir(), "unzip-resource-leak-test")
	assert.Ok(t, err)
	defer os.RemoveAll(tempPath)

	testFile, err := os.Create(filepath.Join(tempPath, "test.zip"))
	assert.Ok(t, err)
	defer testFile.Close()

	zw := zip.NewWriter(testFile)

	// Add some files to the archive.
	var files = make([]struct {
		Name, Body string
	}, 2000)

	for fileNum := range iter.N(2000) {
		files[fileNum] = struct {
			Name, Body string
		}{
			Name: fmt.Sprintf("file-%d.txt", fileNum),
			Body: fmt.Sprintf("This is file number %d\n", fileNum),
		}
	}

	for _, file := range files {
		f, err := zw.Create(file.Name)
		assert.Ok(t, err)

		_, err = f.Write([]byte(file.Body))
		assert.Ok(t, err)
	}

	// Make sure to check the error on Close.
	err = zw.Close()
	assert.Ok(t, err)

	// Open the zip archive for reading.
	destPath := filepath.Join(tempPath, "out")
	os.MkdirAll(destPath, 0777)
	_, err = Unzip(testFile, destPath)
	assert.Ok(t, err)
}

func TestSanitize(t *testing.T) {
	var tests = []struct {
		malicious string
		sanitized string
	}{
		{"../../.././etc/passwd", "etc/passwd"},
		{"../../etc/passwd", "etc/passwd"},
		{"./etc/passwd", "etc/passwd"},
		{"./././etc/passwd", "etc/passwd"},
		{"nonexistant/b/../file.txt", "nonexistant/file.txt"},
		{"abc../def", "abc../def"},
		{"a/b/c/../d", "a/b/d"},
		{"a/../../c", "c"},
		{"...../etc/password", "...../etc/password"},
	}

	for _, test := range tests {
		a := sanitize(test.malicious)
		assert.Equals(t, test.sanitized, a)
	}
}

func calcNumberOfFiles(t *testing.T, searchDir string) int {
	fileList := []string{}

	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileList = append(fileList, path)
		}
		return nil
	})

	if err != nil {
		t.FailNow()
	}

	return len(fileList)
}
