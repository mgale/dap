package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/udhos/equalfile"
)

// compareFiles is the entry point for file comparison, diff reviews and apply patches
func compareFiles(fileAExt fileInfoExtended, fileBExt fileInfoExtended, dryRun bool) error {
	cmp := equalfile.New(nil, equalfile.Options{}) // compare using single mode
	equal, err := cmp.CompareFile(fileAExt.osPathname, fileBExt.osPathname)

	if err != nil {
		logError("Comparing files failed", err)
		return err
	}

	runtimeStats.FilesScanned++
	if equal == true {
		// Files are the same
		return nil
	}

	runtimeStats.FilesWDiff++
	loadFileContent(&fileAExt)
	loadFileContent(&fileBExt)

	resultDiffInfo, err := createDiffs(fileAExt, fileBExt)
	if err != nil {
		return err
	}

	runtimeStats.PatchesApplied += resultDiffInfo.patchesApplied
	runtimeStats.PatchesErrored += resultDiffInfo.patchesFailed
	runtimeStats.PatchesSkipped += (resultDiffInfo.patchesTotal - resultDiffInfo.patchesApplied)

	if resultDiffInfo.patchesFailed > 0 {
		return fmt.Errorf("Errors occurred while patching file, skip file writes: %s", fileAExt.osPathname)
	}

	if dryRun == true {
		fmt.Printf("Dry-run enabled, skipping file writes\n")
		fmt.Println("TESTING1 2 3")
		logError("Test 1 2 3", nil)
		return nil
	}

	fmt.Printf("Dry-run status: %v\n", dryRun)

	// dryrun is off and we have patched the file
	if (dryRun == false) && (resultDiffInfo.patched == true) {
		err := ioutil.WriteFile(fileAExt.osPathname, []byte(fileAExt.fileContent), 0644)
		return err
	}

	return nil
}

func loadFileContent(fileX *fileInfoExtended) error {
	var err error
	fileX.fileContent, err = ioutil.ReadFile(fileX.osPathname)
	if err != nil {
		logError("Reading file failed", err)
		return err
	}

	fileX.fileContentString = string(fileX.fileContent)
	return nil
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
	lookAtPatches, err := reviewDiff(dmp.DiffPrettyTextByLine(diffs), fileAExt.osPathname, fileBExt.osPathname, fileAExt.autoPatch)
	if err != nil {
		return fileDiffInfo, err
	}

	if lookAtPatches == false {
		return fileDiffInfo, nil
	}

	fileAExtUpdated, patchesTotal, patchesFailed, err := handlePatches(dmp, diffs, fileAExt)
	patchesApplied := patchesTotal - patchesFailed

	fileDiffInfo.patchesTotal = patchesTotal
	fileDiffInfo.patchesApplied = patchesApplied
	fileDiffInfo.patchesFailed = patchesFailed

	if patchesApplied > 0 {
		fileAExt = fileAExtUpdated
		fileDiffInfo.patched = true
	}

	fmt.Printf("\nDiffs: %v, Patches: %v, Applied: %v, Failed: %v\n", len(diffs), patchesTotal, patchesApplied, patchesFailed)
	return fileDiffInfo, nil
}

func reviewDiff(mydiffString string, fileAName string, fileBName string, autoPatch bool) (bool, error) {
	fmt.Println("#####################################################")
	fmt.Println("#DiffOutPut: Appling diff to:", fileAName, "from:", fileBName)
	fmt.Println(mydiffString)
	fmt.Println("#####################################################")

	response := false
	if autoPatch {
		fmt.Print("Review patches and apply them? (y/n): AutoAppling")
		response = true
	} else {
		fmt.Print("Review patches and apply them? (y/n):")
		response = askForConfirmation()
	}
	return response, nil
}

func reviewPatchDetailed(patchString string, fileAName string, autoPatch bool) (bool, error) {
	fmt.Println("#####################################################")
	fmt.Println("#PatchOutPut: Appling patch to:", fileAName)
	fmt.Println(patchString)
	fmt.Println("#####################################################")

	response := false
	if autoPatch {
		fmt.Print("Apply patch? (y/n): AutoAppling")
		response = true
	} else {
		fmt.Print("Apply patch? (y/n):")
		response = askForConfirmation()
	}
	return response, nil
}

func handlePatches(dmp *diffmatchpatch.DiffMatchPatch, diffs []diffmatchpatch.Diff, fileAExt fileInfoExtended) (fileInfoExtended, int, int, error) {

	myPatches := dmp.PatchMake(diffs)
	applyPatchList, err := stagePatches(myPatches, fileAExt.osPathname, fileAExt.autoPatch)

	if err != nil {
		fmt.Println(err)
		return fileAExt, 0, 0, err
	}

	fileAtextnew, patchResults := dmp.PatchApply(applyPatchList, fileAExt.fileContentString)

	patchesTotal := 0
	patchesFailed := 0
	for _, patchResult := range patchResults {
		patchesTotal++
		if patchResult == false {
			patchesFailed++
		}
	}

	fileAExt.fileContentString = fileAtextnew
	fileAExt.fileContent = []byte(fileAtextnew)

	return fileAExt, patchesTotal, patchesFailed, err
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
func askForConfirmation() bool {
	var response string

	_, err := fmt.Scanln(&response)
	if err != nil {
		if err.Error() == "unexpected newline" {
			response = ""
		} else {
			logError("Error during confirmation", err)
			return false
		}
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		fmt.Println("I'm sorry but I didn't get what you meant, please type (y)es or (n)o and then press enter:")
		return askForConfirmation()
	}
}
