package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"strings"
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
	newContent     []byte
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

// showFinishedResults takes in an bufio writer like
// os.Stdout for example and writes the results.
func showFinishedResults(output *bufio.Writer, runtimeStats trackedStats) error {
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
		Unsorted: false,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {

			if strings.Contains(osPathname, ".git") {
				return godirwalk.SkipThis
			}

			if strings.Contains(osPathname, ".terraform") {
				return godirwalk.SkipThis
			}

			if de.IsDir() {
				runtimeStats.DirSearched++
			}

			if de.IsRegular() {
				fmt.Print("\033[u\033[K", osPathname)
				fileinfo, _ := os.Stat(osPathname)
				fInfoExt := fileInfoExtended{
					osPathname: osPathname,
					fileInfo:   fileinfo,
				}
				foundFiles = append(foundFiles, fInfoExt)
				runtimeStats.FilesScanned++

			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
	})

	fmt.Print("\033[u\033[KDone\n")
	return foundFiles
}

func mainWork(opt *getoptions.GetOpt, pathAExt fileInfoExtended, pathBExt fileInfoExtended) int {

	runtimeStats.Starttime = time.Now()
	bufferedOutput := bufio.NewWriter(os.Stdout)
	defer bufferedOutput.Flush()

	if pathAExt.fileInfo.IsDir() && pathBExt.fileInfo.IsDir() {
		// We are comparing directories
		pathAFiles := getAllFiles(pathAExt.osPathname)
		pathBFiles := getAllFiles(pathBExt.osPathname)

		fileMapList := []string{}
		fileMap := make(map[string][]fileInfoExtended)
		for _, fileExtInfo := range pathAFiles {
			fileKey := strings.TrimPrefix(fileExtInfo.osPathname, pathAExt.osPathname)
			fileMap[fileKey] = []fileInfoExtended{fileExtInfo}
			fileMapList = append(fileMapList, fileKey)
		}

		for _, fileExtInfo := range pathBFiles {
			fileKey := strings.TrimPrefix(fileExtInfo.osPathname, pathBExt.osPathname)
			if _, ok := fileMap[fileKey]; ok {
				mylist := fileMap[fileKey]
				mylist = append(mylist, fileExtInfo)
				fileMap[fileKey] = mylist
			} else {
				fileMap[fileKey] = []fileInfoExtended{fileExtInfo}
			}
		}

		for _, fileName := range fileMapList {
			if len(fileMap[fileName]) == 2 {
				// Files exist in both dirs
				_, err := compareFiles(fileMap[fileName][0], fileMap[fileName][1], opt.Called("dry-run"), opt.Called("report-only"))
				if err != nil {
					return 1
				}
			}
		}

	} else if pathAExt.fileInfo.IsDir() == false && pathBExt.fileInfo.IsDir() == false {
		// We are comparing two files against each other
		runtimeStats.FilesScanned = 2
		_, err := compareFiles(pathAExt, pathBExt, opt.Called("dry-run"), opt.Called("report-only"))
		if err != nil {
			return 1
		}
	}

	err := showFinishedResults(bufferedOutput, runtimeStats)
	if err != nil {
		return 1
	}
	return 0
}

func main() {
	opt := getoptions.New()
	opt.Bool("help", false, opt.Alias("h", "?"))
	opt.Bool("version", false, opt.Alias("V"))
	opt.Bool("dry-run", false, opt.Description("Dry-run skips updating the underlying file contents"))
	opt.Bool("report-only", false, opt.Alias("q"), opt.Description("Report only files that differ"))
	//opt.Bool("recursive", true, opt.Description("Recursively look for files if inputs are directories"))
	// opt.Bool("follow-sym-links", false, opt.Description("Follow symlinks"))
	// opt.Bool("include-hidden-files", false, opt.Description("Include hidden files and directories"))
	// opt.Bool("report-identical-files", false, opt.Alias("s"), opt.Description("Report only files that are the same"))
	//diffContext := opt.IntOptional("context", 3)

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

	os.Exit(mainWork(opt, pathAExtened, pathBExtened))

}
