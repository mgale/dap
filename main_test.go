package main

import (
	"bufio"
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/DavidGamba/go-getoptions"
)

func Test_showFinishedResults(t *testing.T) {

	runtimeStats := trackedStats{}
	var b bytes.Buffer
	bufferedOutput := bufio.NewWriter(&b)

	type args struct {
		output       *bufio.Writer
		runtimeStats trackedStats
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"EmptyStats", args{output: bufferedOutput, runtimeStats: runtimeStats}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := showFinishedResults(tt.args.output, tt.args.runtimeStats); (err != nil) != tt.wantErr {
				t.Errorf("showFinishedResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_mainWork(t *testing.T) {

	fileA := loadTestFile("tests/same/a/t1.txt")
	fileB := loadTestFile("tests/same/b/t1.txt")

	dirA := loadTestFile("tests/different/hostA")
	dirB := loadTestFile("tests/different/hostB")

	optTest := getoptions.New()
	optTest.Bool("report-only", true)

	type args struct {
		opt      *getoptions.GetOpt
		pathAExt fileInfoExtended
		pathBExt fileInfoExtended
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"Files", args{opt: optTest, pathAExt: fileA, pathBExt: fileB}, 0},
		{"Dirs", args{opt: optTest, pathAExt: dirA, pathBExt: dirB}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mainWork(tt.args.opt, tt.args.pathAExt, tt.args.pathBExt); got != tt.want {
				t.Errorf("mainWork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAllFiles(t *testing.T) {

	fileListDefault := []string{
		"tests/dirwalk/two/two.txt",
		"tests/dirwalk/three/three.txt",
		"tests/dirwalk/four/four.txt",
	}

	fileListHidden := []string{
		"tests/dirwalk/two/two.txt",
		"tests/dirwalk/three/three.txt",
		"tests/dirwalk/four/four.txt",
		"tests/dirwalk/five/.hiddenD/five.txt",
	}

	fileListSymLinks := []string{
		"tests/dirwalk/two/two.txt",
		"tests/dirwalk/three/three.txt",
		"tests/dirwalk/four/four.txt",
	}

	type args struct {
		diffPath string
	}
	tests := []struct {
		name         string
		args         args
		testHidden   bool
		testSymLinks bool
		want         []string
	}{
		{"Defaults", args{diffPath: "tests/dirwalk"}, false, false, fileListDefault},
		{"Hidden", args{diffPath: "tests/dirwalk"}, true, false, fileListHidden},
		{"SymLinks", args{diffPath: "tests/dirwalk"}, false, true, fileListSymLinks},
	}
	for _, tt := range tests {

		includeHidden = false
		followSymLinks = false
		if tt.testHidden {
			includeHidden = true
		}

		if tt.testSymLinks {
			followSymLinks = true
		}

		t.Run(tt.name, func(t *testing.T) {
			got := getAllFiles(tt.args.diffPath)
			myfileList := []string{}
			for _, fileInfo := range got {
				myfileList = append(myfileList, fileInfo.osPathname)
			}
			sort.Strings(myfileList)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(myfileList, tt.want) {
				t.Errorf("getAllFiles() = %v, want %v", myfileList, tt.want)
			}
		})
	}

	includeHidden = false
	followSymLinks = false
}
