package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"
)

func updateStdInContent(tmpfile *os.File, input string) {
	content := []byte(input)
	if _, err := tmpfile.Write(content); err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Seek(0, 0); err != nil {
		log.Fatal(err)
	}

}

func loadTestFile(fileName string) fileInfoExtended {
	fileStat, _ := os.Stat(fileName)
	testFileExt := fileInfoExtended{
		osPathname: fileName,
		fileInfo:   fileStat,
	}

	return testFileExt
}

func Test_askForConfirmation(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }() // Restore original Stdin

	tests := []struct {
		name string
		want bool
	}{
		{"y", true},
		{"Y", true},
		{"yes", true},
		{"n", false},
		{"N", false},
		{"no", false},
		{"blabla", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := ioutil.TempFile("", "utesttmp.txt")
			if err != nil {
				log.Fatal(err)
			}

			os.Stdin = tmpfile

			updateStdInContent(tmpfile, tt.name)
			if got := askForConfirmation(); got != tt.want {
				t.Errorf("askForConfirmation() = %v, want %v", got, tt.want)
			}

			os.Remove(tmpfile.Name())
			if err := tmpfile.Close(); err != nil {
				log.Fatal(err)
			}
		})
	}

}

func Test_reviewDiff(t *testing.T) {
	type args struct {
		mydiffString string
		fileAName    string
		fileBName    string
		autoApply    bool
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"SimpleTest1", args{mydiffString: "Test1", fileAName: "FileA", fileBName: "FileB", autoApply: true}, true, false},
		{"SimpleTest2", args{mydiffString: "Test2", fileAName: "FileA", fileBName: "FileB", autoApply: false}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reviewDiff(tt.args.mydiffString, tt.args.fileAName, tt.args.fileBName, tt.args.autoApply)
			if (err != nil) != tt.wantErr {
				t.Errorf("reviewDiff() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("reviewDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_reviewPatchDetailed(t *testing.T) {
	type args struct {
		patchString string
		fileAName   string
		autoApply   bool
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"SimpleTest1", args{patchString: "Test1", fileAName: "FileA", autoApply: true}, true, false},
		{"SimpleTest2", args{patchString: "Test2", fileAName: "FileA", autoApply: false}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reviewPatchDetailed(tt.args.patchString, tt.args.fileAName, tt.args.autoApply)
			if (err != nil) != tt.wantErr {
				t.Errorf("reviewPatchDetailed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("reviewPatchDetailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadFileContent(t *testing.T) {

	fileA := loadTestFile("tests/same/a/t1.txt")
	fileFake := loadTestFile("tests/fakefile")

	type args struct {
		fileX *fileInfoExtended
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"RealFile", args{fileX: &fileA}, false},
		{"FakeFile", args{fileX: &fileFake}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := loadFileContent(tt.args.fileX); (err != nil) != tt.wantErr {
				t.Errorf("loadFileContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_compareFiles(t *testing.T) {

	fileA := loadTestFile("tests/same/a/t1.txt")
	fileB := loadTestFile("tests/same/b/t1.txt")
	fileC := loadTestFile("tests/smalldiff/t2.txt")
	fileFake := loadTestFile("tests/fakefile")

	fileSource := loadTestFile("tests/smalldiff/t2.txt")

	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		log.Fatal(err)
	}

	data, _ := ioutil.ReadFile("tests/smalldiff/t1.txt")
	err = ioutil.WriteFile(tmpfile.Name(), data, 0644)

	filePatch := loadTestFile(tmpfile.Name())
	filePatch.autoPatch = true
	defer os.Remove(tmpfile.Name()) // clean up

	type args struct {
		fileAExt fileInfoExtended
		fileBExt fileInfoExtended
		dryRun   bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"FilesMatch", args{fileAExt: fileA, fileBExt: fileB, dryRun: true}, false},
		{"FilesDifer", args{fileAExt: fileA, fileBExt: fileC, dryRun: true}, false},
		{"FilesSame", args{fileAExt: fileA, fileBExt: fileA, dryRun: true}, false},
		{"FakeFile", args{fileAExt: fileA, fileBExt: fileFake, dryRun: true}, true},
		{"RealPatch", args{fileAExt: filePatch, fileBExt: fileSource, dryRun: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := compareFiles(tt.args.fileAExt, tt.args.fileBExt, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("compareFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	fmt.Printf("Result: %+v", runtimeStats)
}

func Test_createDiffs(t *testing.T) {

	fileA := loadTestFile("tests/same/a/t1.txt")
	loadFileContent(&fileA)
	// fileB := loadTestFile("tests/same/b/t1.txt")
	// loadFileContent(&fileB)
	fileC := loadTestFile("tests/smalldiff/t2.txt")
	loadFileContent(&fileC)

	fileSource := loadTestFile("tests/smalldiff/t2.txt")
	loadFileContent(&fileSource)

	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		log.Fatal(err)
	}

	data, _ := ioutil.ReadFile("tests/smalldiff/t1.txt")
	err = ioutil.WriteFile(tmpfile.Name(), data, 0644)

	filePatch := loadTestFile(tmpfile.Name())
	loadFileContent(&filePatch)
	filePatch.autoPatch = true
	defer os.Remove(tmpfile.Name()) // clean up

	noChangesDiff := fileDiffInfo{
		diffCount:      13,
		patchesTotal:   0,
		patchesApplied: 0,
		patchesFailed:  0,
		patched:        false,
	}

	withChangesDiff := noChangesDiff
	withChangesDiff.diffCount = 13
	withChangesDiff.patchesTotal = 4
	withChangesDiff.patchesApplied = 4
	withChangesDiff.patched = true

	type args struct {
		fileAExt fileInfoExtended
		fileBExt fileInfoExtended
	}
	tests := []struct {
		name    string
		args    args
		want    fileDiffInfo
		wantErr bool
	}{
		{"FilesDifer", args{fileAExt: fileA, fileBExt: fileC}, noChangesDiff, false},
		{"RealPatch", args{fileAExt: filePatch, fileBExt: fileSource}, withChangesDiff, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createDiffs(tt.args.fileAExt, tt.args.fileBExt)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDiffs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createDiffs() = %v, want %v", got, tt.want)
			}
		})
	}
}
