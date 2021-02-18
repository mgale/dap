package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/DavidGamba/go-getoptions"
	"github.com/karrick/godirwalk"
)

const semVersion = "0.0.6"

var diffContext int
var ignorePaths []string
var includeHidden bool = false
var followSymLinks bool = false
var enableDebugLogs bool = false

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
		fmt.Fprint(os.Stderr, err.Error())
	}
}

func logDebug(myMsg string) {
	if enableDebugLogs {
		fmt.Printf("Debug: %v\n", myMsg)
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

			logDebug("Checking file:" + osPathname)
			if strings.Contains(osPathname, "/.") {
				if !includeHidden {
					logDebug("Hidden check: Ignoring:" + osPathname)
					return godirwalk.SkipThis
				}
			}

			if de.IsSymlink() {
				if !followSymLinks {
					return godirwalk.SkipThis
				}
			}

			for _, ignorePath := range ignorePaths {
				if strings.Contains(osPathname, ignorePath) {
					return godirwalk.SkipThis
				}
			}

			if de.IsDir() {
				runtimeStats.DirSearched++
			}

			if de.IsRegular() {
				fileinfo, _ := os.Stat(osPathname)
				fInfoExt := fileInfoExtended{
					osPathname: osPathname,
					fileInfo:   fileinfo,
				}
				logDebug("Including file:" + osPathname)
				foundFiles = append(foundFiles, fInfoExt)
				runtimeStats.FilesScanned++

			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
	})

	return foundFiles
}

func mainWork(opt *getoptions.GetOpt, pathAExt fileInfoExtended, pathBExt fileInfoExtended) int {

	runtimeStats.Starttime = time.Now()
	bufferedOutput := bufio.NewWriter(os.Stdout)
	defer bufferedOutput.Flush()

	if pathAExt.fileInfo.IsDir() && pathBExt.fileInfo.IsDir() {
		// We are comparing directories
		pathAExt.osPathname = filepath.Clean(pathAExt.osPathname)
		pathBExt.osPathname = filepath.Clean(pathBExt.osPathname)
		pathAFiles := getAllFiles(pathAExt.osPathname)
		pathBFiles := getAllFiles(pathBExt.osPathname)

		fileMapList := []string{}
		fileMap := make(map[string][]fileInfoExtended)
		for _, fileExtInfo := range pathAFiles {
			fileKey := strings.TrimPrefix(fileExtInfo.osPathname, pathAExt.osPathname)
			fileMap[fileKey] = []fileInfoExtended{fileExtInfo}
			fileMapList = append(fileMapList, fileKey)
			logDebug("Primary path:" + fileKey)
		}

		for _, fileExtInfo := range pathBFiles {
			fileKey := strings.TrimPrefix(fileExtInfo.osPathname, pathBExt.osPathname)
			logDebug("Secondary path:" + fileKey)
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
				logDebug("Comparing file:" + fileName)
				_, err := compareFiles(fileMap[fileName][0], fileMap[fileName][1], opt.Called("dry-run"), opt.Called("report-only"))
				if err != nil {
					return 1
				}
			} else {
				logDebug("Skipping file:" + fileName)
			}
		}

	} else if !pathAExt.fileInfo.IsDir() && !pathBExt.fileInfo.IsDir() {
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

func program(args []string) int {

	opt := getoptions.New()
	opt.Self("", `Transforms <original> into <desired_changes>. Said another way, brings changes into <original> from <desired_changes>.

Example: ./dap original desired_changes`)
	opt.HelpSynopsisArgs("<original> <desired_changes>")
	opt.Bool("help", false, opt.Alias("h", "?"))
	opt.Bool("version", false, opt.Alias("V"))
	opt.BoolVar(&enableDebugLogs, "debug", false)
	opt.Bool("dry-run", false, opt.Description("Dry-run skips updating the underlying file contents"))
	opt.Bool("report-only", false, opt.Alias("q"), opt.Description("Report only files that differ"))
	opt.StringSliceVar(&ignorePaths, "ignore-paths", 1, 1, opt.Description("Excludes pathnames from directory search, providing a value overrides the defaults of .git and .terraform"))
	opt.BoolVar(&includeHidden, "include-hidden", false, opt.Description("Include hidden files and directories"))
	opt.BoolVar(&followSymLinks, "follow-sym-links", false, opt.Description("Follow symlinks"))
	// opt.Bool("report-identical-files", false, opt.Alias("s"), opt.Description("Report only files that are the same"))
	//diffContext := opt.IntOptional("context", 3)

	remaining, err := opt.Parse(args)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		fmt.Fprint(os.Stderr, opt.Help(getoptions.HelpSynopsis))
		return 2
	}

	if opt.Called("help") {
		fmt.Fprint(os.Stderr, opt.Help())
		return 0
	}
	if opt.Called("version") {
		fmt.Println(semVersion)
		return 0
	}

	if len(remaining) != 2 {
		fmt.Fprintf(os.Stderr, "ERROR: Missing required arguments!\n")
		fmt.Fprint(os.Stderr, opt.Help())
		return 2
	}

	pathA, err := os.Stat(remaining[0])
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error, No such file or directory: %s\n", remaining[0])
		return 127
	}

	pathB, err := os.Stat(remaining[1])
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error, No such file or directory: %s\n", remaining[1])
		return 127
	}

	pathAExtened := fileInfoExtended{osPathname: remaining[0], fileInfo: pathA}
	pathBExtened := fileInfoExtended{osPathname: remaining[1], fileInfo: pathB}

	return mainWork(opt, pathAExtened, pathBExtened)
}

func main() {

	os.Exit(program(os.Args[1:]))

}
