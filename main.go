package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"text/tabwriter"
	"time"

	"github.com/DavidGamba/go-getoptions"
	"github.com/karrick/godirwalk"
)

const semVersion = "0.0.1"

var diffContext int

type trackedStats struct {
	FilesScanned   int
	FilesWDiff     int
	DirSearched    int
	PatchesApplied int
	PatchesSkipped int
	PatchesErrored int
	Starttime      time.Time
	Duration       string
}

type fileInfoExtended struct {
	fileInfo          os.FileInfo
	osPathname        string
	fileContent       []byte
	fileContentString string
	autoPatch         bool
}

type fileDiffInfo struct {
	diffCount      int
	patchesTotal   int
	patchesApplied int
	patchesFailed  int
	patched        bool
}

var runtimeStats trackedStats

var finishedResponse = `Scanned:{{"\t"}}Files: {{.FilesScanned}}{{"\t"}}Directories: {{.DirSearched}}{{"\t"}}Diffs: {{.FilesWDiff}}{{"\t"}}Patched: {{.PatchesApplied}}{{"\t"}}Skipped: {{.PatchesSkipped}}{{"\t"}}Errors: {{.PatchesErrored}} {{"\t"}}Runtime: {{.Duration}}
`
var finishedTpl = template.Must(template.New("finishedReponse").Parse(finishedResponse))

// logError responsibility is to output or log the Error only.
// The pattern is to call logError if err != nil so an
// intelligent error message can be presented to the user.
func logError(myMsg string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", myMsg)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
}

// func checkDiffPath(pathName string) error {
// 	_, err := os.Stat(pathName)
// 	if os.IsNotExist(err) {
// 		logError("File does not exits", err)
// 		return err
// 	}

// 	return nil
// }

// showFinishedResults takes in an bufio writer like
// os.Stdout for example and writes the results.
func showFinishedResults(output *bufio.Writer) error {
	runtimeStats.Duration = time.Since(runtimeStats.Starttime).String()

	w := tabwriter.NewWriter(output, 8, 8, 8, ' ', 0)
	err := finishedTpl.Execute(w, runtimeStats)
	if err != nil {
		logError("Executing template", err)
		return err
	}

	w.Flush()

	return nil
}

func getAllFiles(diffPath string) []fileInfoExtended {
	foundFiles := []fileInfoExtended{}
	fmt.Println("Loading files from ", diffPath)
	godirwalk.Walk(diffPath, &godirwalk.Options{
		Unsorted: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			fmt.Print("\033[u\033[K", osPathname)
			fileinfo, _ := os.Stat(osPathname)
			fInfoExt := fileInfoExtended{
				osPathname: osPathname,
				fileInfo:   fileinfo,
			}
			foundFiles = append(foundFiles, fInfoExt)
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
	})

	fmt.Print("\033[u\033[KDone\n")
	return foundFiles
}

func mainWork(opt *getoptions.GetOpt, diffContext int, pathAExt fileInfoExtended, pathBExt fileInfoExtended) int {

	runtimeStats.Starttime = time.Now()
	bufferedOutput := bufio.NewWriter(os.Stdout)
	defer bufferedOutput.Flush()

	if pathAExt.fileInfo.IsDir() && pathBExt.fileInfo.IsDir() {
		// We are comparing directories
	} else if pathAExt.fileInfo.IsDir() == false && pathBExt.fileInfo.IsDir() == false {
		// We are comparing two files against each other
		err := compareFiles(pathAExt, pathBExt, opt.Called("dry-run"))
		if err != nil {
			return 1
		}
	} else {
		// We have 1 file and 1 dir, append the basename of the file onto the directory
		fmt.Println("Currently not supported")
	}

	//diffInfo := []fileDiffInfo{}
	// if diffDirCount > 0 {
	// 	for _, myfInfoExt := range getAllFiles(diffPaths[0]) {
	// 		fDiffInfo := fileDiffInfo{fileAInfo: myfInfoExt}
	// 		diffInfo[myfInfoExt.osPathname] = fDiffInfo
	// 	}
	// 	for _, myfInfoExt := range getAllFiles(diffPaths[1]) {
	// 		fDiffInfo := fileDiffInfo{fileAInfo: myfInfoExt}
	// 		diffInfo[myfInfoExt.osPathname] = fDiffInfo
	// 	}

	// }

	// All work completed, output runtime stats and exit
	if opt.Called("brief") == false {
		err := showFinishedResults(bufferedOutput)
		if err != nil {
			return 1
		}
	}
	return 0
}

func main() {
	opt := getoptions.New()
	opt.Bool("help", false, opt.Alias("h", "?"))
	opt.Bool("version", false, opt.Alias("V"))
	opt.Bool("dry-run", false, opt.Description("Dry-run skips updating the underlying file contents"))
	opt.Bool("brief", false, opt.Alias("q"), opt.Description("Report only files that differ"))
	opt.Bool("recursive", true, opt.Description("Recursively look for files if inputs are directories"))
	// opt.Bool("follow-sym-links", false, opt.Description("Follow symlinks"))
	// opt.Bool("include-hidden-files", false, opt.Description("Include hidden files and directories"))
	// opt.Bool("report-identical-files", false, opt.Alias("s"), opt.Description("Report only files that are the same"))
	diffContext := opt.IntOptional("context", 3)

	remaining, err := opt.Parse(os.Args[1:])
	if opt.Called("help") {
		fmt.Fprintf(os.Stderr, opt.Help())
		os.Exit(1)
	}
	if opt.Called("version") {
		fmt.Println(semVersion)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		fmt.Fprintf(os.Stderr, opt.Help(getoptions.HelpSynopsis))
		os.Exit(1)
	}

	if len(remaining) != 2 {
		fmt.Fprintf(os.Stderr, "Missing arguments to diff, allowed 2 !!!\n")
		fmt.Fprintf(os.Stderr, opt.Help())
		os.Exit(1)
	}

	pathA, err := os.Stat(remaining[0])
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error, No such file or directory: %s\n", remaining[0])
		os.Exit(1)
	}

	pathB, err := os.Stat(remaining[1])
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error, No such file or directory: %s\n", remaining[1])
		os.Exit(1)
	}

	pathAExtened := fileInfoExtended{osPathname: remaining[0], fileInfo: pathA}
	pathBExtened := fileInfoExtended{osPathname: remaining[1], fileInfo: pathB}

	os.Exit(mainWork(opt, *diffContext, pathAExtened, pathBExtened))

}
