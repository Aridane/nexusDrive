package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	//"syscall"
	//"os/exec"

	"github.com/manifoldco/promptui"
	nexusrm "github.com/sonatype-nexus-community/gonexus/rm"
)

var nexusUser string
var nexusPassword string
var selectedRepo string
var nexusServer string

// DownloadFile download a file
func DownloadFile(path string, url string) error {
	syscall.Umask(0)
	err := os.MkdirAll(filepath.Dir(path), 0775)

	out, err := os.Create(path)
	if err != nil {
		fmt.Println("Error", err)
		return err
	}
	defer out.Close()

	index := 7
	if strings.Contains(url, "https") {
		index = 8
	}
	newURL := url[:index] + nexusUser + ":" + nexusPassword + "@" + url[index:]
	fmt.Println("url", newURL)
	resp, err := http.Get(newURL)
	if err != nil {
		fmt.Println("Error", err)
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func promtInput(promtStr, defaultValue string) (string, error) {
	value := defaultValue
	prompt := promptui.Prompt{
		Label: promtStr + "(" + defaultValue + ")",
	}
	result, err := prompt.Run()

	if err == nil && result != "" {
		value = result
	}
	return value, err
}

func promtInputMasked(promtStr, defaultValue string) (string, error) {
	value := defaultValue
	prompt := promptui.Prompt{
		Label: promtStr + "(" + defaultValue + ")",
		Mask:  '*'}
	result, err := prompt.Run()

	if err == nil && result != "" {
		value = result
	}
	return value, err
}

func promtSelect(promtStr string, options []string, defaultValue string) (string, error) {
	value := defaultValue
	promptSel := promptui.Select{
		Label: promtStr + "(" + defaultValue + ")",
		Items: options,
	}

	_, result, err := promptSel.Run()

	if err == nil && result != "" {
		value = result
	}
	return value, err
}

//ListRepo List all files in repo
func ListRepo(rm nexusrm.RM, repo string) {
	components, err := nexusrm.GetComponents(rm, repo)
	if err == nil {
		for _, c := range components {
			fmt.Println(c.Name, c.Assets[0].Checksum.Md5)
		}
	}
}

func getLocalFiles(rootPath string) ([]string, error) {
	var res []string
	err := filepath.Walk(rootPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				relPath, _ := filepath.Rel(rootPath, path)
				res = append(res, relPath)
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
	return res, err
}

// ListLocal List all files in local folder
func ListLocal(rootPath string) {
	files, err := getLocalFiles(rootPath)
	if err != nil {
		log.Println(err)
	}
	for f := range files {
		fmt.Println(f)
	}
}

func getComponent(rm nexusrm.RM, repo string, path string) (nexusrm.RepositoryItem, bool) {
	var res nexusrm.RepositoryItem
	components, err := nexusrm.GetComponents(rm, repo)
	found := false
	if err == nil {
		for _, c := range components {
			if c.Assets[0].Path == path {
				res = c
				found = true
				break
			}
		}
	}
	return res, found
}

// getMd5 Gets md5, 0 if error
func getMd5(path string) (string, error) {
	res := ""
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return res, err
	}
	res = fmt.Sprintf("%x", h.Sum(nil))
	return res, nil
}

// DownloadFolder Download all files from repo overwriting changed ones
func DownloadFolder(rm nexusrm.RM, repo string, source string, destination string) {
	fmt.Println("Download from ", repo, " source ", source, " dst ", destination)
	components, err := nexusrm.GetComponents(rm, repo)
	if err == nil {
		for _, c := range components {
			matched := strings.HasPrefix(c.Name, source)
			if err == nil && matched {
				fmt.Println(c.Name)
				DownloadIfDifferent(source, destination, c)
			}
		}
	}
}

// DownloadIfDifferent Downloads repository item if md5 are different
func DownloadIfDifferent(sourcePath string, destination string, c nexusrm.RepositoryItem) {
	cName := strings.TrimPrefix(c.Name, sourcePath)
	fileMd5, err := getMd5(filepath.Join(destination, cName))
	if os.IsNotExist(err) || (err == nil && fileMd5 != c.Assets[0].Checksum.Md5) {
		fmt.Println("Downloading to ", filepath.Join(destination, cName))
		fmt.Println("\tRemote md5 ", c.Assets[0].Checksum.Md5)
		fmt.Println("\tLocal md5  ", fileMd5)
		DownloadFile(filepath.Join(destination, cName), c.Assets[0].DownloadURL)
	} else {
		if err != nil {
			fmt.Println("Skipping ", c.Name, err)
		}
		if fileMd5 == c.Assets[0].Checksum.Md5 {
			fmt.Println("Skipping ", c.Name, ": Unchanged")
		}
	}
}

// UploadIfDifferent Uploads item if md5 are different
func UploadIfDifferent(rm nexusrm.RM, repo string, source string, destination string) {

	fmt.Println("UploadIfDifferent repo", repo, " source ", source, " destination ", destination)

	c, found := getComponent(rm, repo, destination)

	if found {
		fmt.Println("Found")
		fileMd5, err := getMd5(source)
		if err == nil && fileMd5 != c.Assets[0].Checksum.Md5 {
			fmt.Println("Uploading ", source, " to ", destination)
			UploadFile(rm, repo, source, destination)
		} else {
			if err != nil {
				fmt.Println("Skipping ", c.Name, err)
			}
			if fileMd5 == c.Assets[0].Checksum.Md5 {
				fmt.Println("Skipping ", c.Name, ": Unchanged")
			}
		}
	} else {
		fmt.Println("Not found")
		fmt.Println("Uploading ", source, " to ", destination)
		UploadFile(rm, repo, source, destination)
	}

}

// UploadFile uploads a file to repo
func UploadFile(rm nexusrm.RM, repo string, source string, path string) {

	/*f, err := os.Open(source)
	if err != nil {
		log.Fatal(err)
	}*/

	cmd := exec.Command("curl", "-O", "-k", "-u", nexusUser+":"+nexusPassword,
		"-H", "\"Content-type: application/json\"",
		"-H", "\"Expect:\"",
		"--upload-file", source,
		nexusServer+"/repository/"+selectedRepo+"/"+path)

	fmt.Println("curl", "-O", "-k", "-u", nexusUser+":"+nexusPassword,
		"--upload-file", source,
		nexusServer+"/repository/"+selectedRepo+"/"+path)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error: ", err)
	}
	/*
		component := nexusrm.UploadComponentRaw{
			Assets: []nexusrm.UploadAssetRaw{
				nexusrm.UploadAssetRaw{
					File:     f,
					Filename: filepath.Base(path)}},
			Directory: filepath.Dir(path),
			Tag:       ""}
		err = nexusrm.UploadComponent(rm, repo, component)
		if err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()*/
}

// UploadFolder files overwriting
func UploadFolder(rm nexusrm.RM, repo string, source string, destination string) {
	files, err := getLocalFiles(source)
	if err != nil {
		log.Println(err)
	}
	for _, f := range files {
		fmt.Println(f)
		UploadIfDifferent(rm, repo, filepath.Join(source, f), filepath.Join(destination, f))
	}
}

// DownloadOne downloads a single file
func DownloadOne(rm nexusrm.RM, repo string, rootPath string) {
	// Select file interactively
	components, err := nexusrm.GetComponents(rm, repo)

	if err != nil {
		log.Fatal(err)
		return
	}

	if len(components) == 0 {
		fmt.Println("No components")
		return
	}
	var componentNames []string
	var componentMap map[string]nexusrm.RepositoryItem
	componentMap = make(map[string]nexusrm.RepositoryItem)
	if err == nil {
		for _, c := range components {
			componentNames = append(componentNames, c.Name)
			componentMap[c.Name] = c
		}
	}
	componentNames = append(componentNames, "Back")
	// Download file
	selectedComponent, _ := promtSelect("Select file to download:", componentNames, "")

	if selectedComponent == "Back" {
		return
	}

	DownloadIfDifferent("", rootPath, componentMap[selectedComponent])

}

// UploadOne uploads a single file
func UploadOne(rm nexusrm.RM, repo string, source string, destination string) {
	// Select file interactively
	files, err := getLocalFiles(source)
	if err != nil {
		log.Println(err)
	}
	selectedFile, _ := promtSelect("Select file to upload:", files, "")

	// Upload file
	UploadIfDifferent(rm, repo, filepath.Join(source, selectedFile), filepath.Join(destination, selectedFile))
}

func main() {

	exit := false
	back := false
	rootPath := ""
	var err error
	nexusServer, err = promtInput("Server: ", "https://pforgeipt.intra.airbusds.corp/nexus3")
	nexusUser, err = promtInput("User: ", "c84370")
	nexusPassword, err = promtInputMasked("Password: ", "admin")

	// Connect to nexus
	rm, err := nexusrm.New(nexusServer, nexusUser, nexusPassword)
	if err != nil {
		panic(err)
	}
	fmt.Println("rm created")

	for !exit {
		// Get list of repos
		var repoNames []string
		repos, err := nexusrm.GetRepositories(rm)
		if err == nil {
			for _, r := range repos {
				repoNames = append(repoNames, r.Name)
			}
		} else {
			log.Fatal(err)
		}

		repoNames = append(repoNames, "Exit")
		// Select which repo to work with
		selectedRepo, _ = promtSelect("Select Repo", repoNames, "")
		if selectedRepo == "Exit" {
			exit = true
		}
		for !exit && !back {
			actions := []string{"List repo", "List local", "Download folder", "Upload folder",
				"Download file", "Upload file", "Back", "Exit"}
			action, _ := promtSelect("Select Actions", actions, "")

			switch action {
			case actions[0]:
				ListRepo(rm, selectedRepo)
			case actions[1]:
				rootPath, err := promtInput("Path: ", "/home/user/somefolder")
				if err != nil {
					panic(err)
				}
				ListLocal(rootPath)
			case actions[2]:
				source, err := promtInput("Source: ", "somefolder/somefolder")
				if err != nil {
					panic(err)
				}
				destination, err := promtInput("Destination: ", "/home/user/somefolder")
				if err != nil {
					panic(err)
				}
				DownloadFolder(rm, selectedRepo, source, destination)
			case actions[3]:
				source, err := promtInput("Source: ", "/media/c84370/Data/NEXUS/")
				if err != nil {
					panic(err)
				}
				destination, err := promtInput("Destination: ", "releases/")
				if err != nil {
					panic(err)
				}
				UploadFolder(rm, selectedRepo, source, destination)
			case actions[4]:
				DownloadOne(rm, selectedRepo, rootPath)
			case actions[5]:
				source, err := promtInput("Source: ", "/home/user/somefolder")
				if err != nil {
					panic(err)
				}
				destination, err := promtInput("Destination: ", "somefolder/somefolder")
				if err != nil {
					panic(err)
				}
				UploadOne(rm, selectedRepo, source, destination)
			case actions[len(actions)-2]:
				back = true
			case actions[len(actions)-1]:
				exit = true
			}
		}
		back = false
	}
}
