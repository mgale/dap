package main

import (
	"bufio"
	"bytes"
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
