package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/gookit/color"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/udhos/equalfile"
)

// ErrorCanceled is returned when the user decideds to quit.
var ErrorCanceled = fmt.Errorf("canceled by user")

// compareFiles is the entry point for file comparison, diff reviews and apply patches
// TBD: Currently the match result is returned, not sure if we need this or not.
func compareFiles(fileAExt fileInfoExtended, fileBExt fileInfoExtended, dryRun bool, reportOnly bool) (bool, error) {
	cmp := equalfile.New(nil, equalfile.Options{}) // compare using single mode
	equal, err := cmp.CompareFile(fileAExt.osPathname, fileBExt.osPathname)

	if err != nil {
		logError("Comparing files failed", err)
		return false, err
	}

	if reportOnly && !equal {
		runtimeStats.FilesWDiff++
		fmt.Printf("Files %s and %s differ\n", fileAExt.osPathname, fileBExt.osPathname)
		return equal, nil
	}

	if equal {
		// Files are the same
		return equal, nil
	}

	runtimeStats.FilesWDiff++
	loadFileContent(&fileAExt)
	loadFileContent(&fileBExt)

	resultDiffInfo, err := createDiffs(fileAExt, fileBExt)
	if err != nil {
		return equal, err
	}

	runtimeStats.PatchesApplied += resultDiffInfo.patchesApplied
	runtimeStats.PatchesErrored += resultDiffInfo.patchesFailed
	runtimeStats.PatchesSkipped += (resultDiffInfo.patchesTotal - resultDiffInfo.patchesApplied)

	if resultDiffInfo.patchesFailed > 0 {
		return equal, fmt.Errorf("while patching file, skip file writes: %s", fileAExt.osPathname)
	}

	if dryRun {
		fmt.Printf("Dry-run enabled, skipping file writes: %s\n", fileAExt.osPathname)
		return equal, nil
	}

	// dryrun is off and we have patched the file
	if !dryRun && resultDiffInfo.patched {
		err := ioutil.WriteFile(fileAExt.osPathname, resultDiffInfo.newContent, 0644)
		return equal, err
	}

	return equal, nil
}

// The files have already be read by a quick compare utility
// If we get an I/O error here we should just exit.
func loadFileContent(fileX *fileInfoExtended) {
	var err error
	fileX.fileContent, err = ioutil.ReadFile(fileX.osPathname)
	if err != nil {
		log.Fatalf("Error reading file: %v, %v", fileX.osPathname, err)
	}

	fileX.fileContentString = string(fileX.fileContent)
}

func splitLines(s string) []string {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

// ColorDiff Returns the diff to the end user with colour
func ColorDiff(diffs []diffmatchpatch.Diff) string {
	out := ""
	for i, diff := range diffs {
		if i == 0 {
			out += color.Style{color.Blue}.Sprint("---\n")
		}
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			out += color.Style{color.Green}.Sprint(strings.ReplaceAll(diff.Text, "\t", "˲   "))
		case diffmatchpatch.DiffDelete:
			out += color.Style{color.Red}.Sprint(strings.ReplaceAll(diff.Text, "\t", "˲   "))
		case diffmatchpatch.DiffEqual:
			lines := splitLines(diff.Text)
			if len(lines) > 0 &&
				// Avoid just adding the beginning of the file as context if it is not close to the diff
				!(i == 0 && len(lines) > 2) {

				// TODO: Control the output of tab with an option
				out += color.Style{color.White}.Sprint(strings.ReplaceAll(lines[0], "\t", "˲   "))
				if len(lines) > 1 {
					out += "\n"
				}
				if len(lines) > 2 {
					out += color.Style{color.Blue}.Sprint("\n---\n")
				}
			}
			if len(lines) > 1 &&
				// Avoid just adding the end of the file as context if it is not close to the diff
				!(i+1 == len(diffs) && len(lines) > 2) {

				// TODO: Control the output of tab with an option
				out += color.Style{color.White}.Sprint(strings.ReplaceAll(lines[len(lines)-1], "\t", "˲   "))
				if strings.HasSuffix(diff.Text, "\n") {
					out += "\n"
				}
			}
		}
	}
	return out
}

// Get a list of Patches / Chunks
func createDiffs(fileAExt fileInfoExtended, fileBExt fileInfoExtended) (fileDiffInfo, error) {

	fileDiffInfo := fileDiffInfo{}

	dmp := diffmatchpatch.New()
	dmp.MatchMaxBits = 100

	// create the diffs between files
	fileAdmp, fileBdmp, dmpStrings := dmp.DiffLinesToChars(fileAExt.fileContentString, fileBExt.fileContentString)
	diffs := dmp.DiffMain(fileAdmp, fileBdmp, false)
	diffs = dmp.DiffCharsToLines(diffs, dmpStrings)
	diffs = dmp.DiffCleanupSemantic(diffs)

	fileDiffInfo.diffCount = len(diffs)
	//review the diff with the user
	lookAtPatches, err := reviewDiff(ColorDiff(diffs), fileAExt.osPathname, fileBExt.osPathname, fileAExt.autoPatch)
	if err != nil {
		return fileDiffInfo, err
	}

	if !lookAtPatches {
		return fileDiffInfo, nil
	}

	fileContent, patchesTotal, patchesFailed, err := handlePatches(dmp, diffs, fileAExt)
	patchesApplied := patchesTotal - patchesFailed

	fileDiffInfo.patchesTotal = patchesTotal
	fileDiffInfo.patchesApplied = patchesApplied
	fileDiffInfo.patchesFailed = patchesFailed

	if patchesApplied > 0 {
		fileDiffInfo.patched = true
		fileDiffInfo.newContent = fileContent
	}

	logError("Error applying patching", err)
	fmt.Printf("\nDiffs: %v, Patches: %v, Applied: %v, Failed: %v\n", len(diffs), patchesTotal, patchesApplied, patchesFailed)
	return fileDiffInfo, nil
}

func reviewDiff(mydiffString string, fileAName string, fileBName string, autoPatch bool) (bool, error) {
	color.Style{color.OpBold}.Printf("Appling diff to: %s, from: %s\n", fileAName, fileBName)
	fmt.Println(mydiffString)

	response := false
	if autoPatch {
		fmt.Print("Review patches and apply them [y,n,q]? AutoAppling")
		response = true
	} else {
		color.Style{color.Blue, color.OpBold}.Print("Review patches and apply them [y,n,q]? ")
		rsp, err := askForConfirmation()
		if err != nil {
			if errors.Is(err, ErrorCanceled) {
				return rsp, err
			}
		}
		response = rsp
	}
	return response, nil
}

func reviewPatchDetailed(patchString string, fileAName string, autoPatch bool) (bool, error) {
	color.Style{color.OpBold}.Printf("Appling diff to: %s\n", fileAName)
	fmt.Println(patchString)

	response := false
	if autoPatch {
		fmt.Print("Apply patch [y,n,q]? AutoAppling")
		response = true
	} else {
		color.Style{color.Blue, color.OpBold}.Print("Apply patch [y,n,q]? ")
		rsp, err := askForConfirmation()
		if err != nil {
			if errors.Is(err, ErrorCanceled) {
				return rsp, err
			}
		}
		response = rsp
	}
	return response, nil
}

func handlePatches(dmp *diffmatchpatch.DiffMatchPatch, diffs []diffmatchpatch.Diff, fileAExt fileInfoExtended) ([]byte, int, int, error) {

	myPatches := dmp.PatchMake(diffs)
	applyPatchList, err := stagePatches(myPatches, fileAExt.osPathname, fileAExt.autoPatch)

	if err != nil {
		fmt.Println(err)
		return nil, 0, 0, err
	}

	fileAtextnew, patchResults := dmp.PatchApply(applyPatchList, fileAExt.fileContentString)

	patchesTotal := 0
	patchesFailed := 0
	for _, patchResult := range patchResults {
		patchesTotal++
		if !patchResult {
			patchesFailed++
		}
	}

	fileContent := []byte(fileAtextnew)

	return fileContent, patchesTotal, patchesFailed, err
}

// Cycles through the patches and returns the patches the User has flagged to be applied.
func stagePatches(myPatches []diffmatchpatch.Patch, fileAName string, autoPatch bool) ([]diffmatchpatch.Patch, error) {

	applyPatchList := []diffmatchpatch.Patch{}

	for _, patch := range myPatches {
		addChunk, err := reviewPatchDetailed(patch.StringByLine(), fileAName, autoPatch)
		if err != nil {
			logError("Error reviewing patch", err)
			return applyPatchList, err
		}
		if addChunk {
			applyPatchList = append(applyPatchList, patch)
		}
	}

	return applyPatchList, nil
}

// Helper method to ask for confirmation from a User
func askForConfirmation() (bool, error) {
	var response string

	_, err := fmt.Scanln(&response)
	if err != nil {
		if err.Error() == "unexpected newline" {
			response = ""
		} else {
			logError("Error during confirmation", err)
			// TODO: Should this still be nil?
			return false, nil
		}
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "q", "quit":
		return false, ErrorCanceled
	default:
		fmt.Print(`y - patch this hunk
n - do not patch this hunk
q - quit; do not patch this hunk or any of the remaining ones
`)
		return askForConfirmation()
	}
}
