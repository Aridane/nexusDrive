package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"

	"github.com/manifoldco/promptui"
	nexusrm "github.com/sonatype-nexus-community/gonexus/rm"
)

// DownloadFile download a file
func DownloadFile(path string, url string) error {
	syscall.Umask(0)
	err := os.MkdirAll(filepath.Dir(path), 0775)

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
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
	var res string
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

// DownloadAll Download all files from repo overwriting changed ones
func DownloadAll(rm nexusrm.RM, repo string, rootPath string) {
	components, err := nexusrm.GetComponents(rm, repo)
	if err == nil {
		for _, c := range components {
			DownloadIfDifferent(rootPath, c)
		}
	}

}

// DownloadIfDifferent Downloads repository item if md5 are different
func DownloadIfDifferent(rootPath string, c nexusrm.RepositoryItem) {
	fileMd5, err := getMd5(filepath.Join(rootPath, c.Name))
	if err == nil && fileMd5 != c.Assets[0].Checksum.Md5 {
		fmt.Println("Downloading ", filepath.Join(rootPath, c.Name))
		fmt.Println("\tRemote md5 ", c.Assets[0].Checksum.Md5)
		fmt.Println("\tLocal md5  ", fileMd5)
		DownloadFile(filepath.Join(rootPath, c.Name), c.Assets[0].DownloadURL)
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
func UploadIfDifferent(rm nexusrm.RM, repo string, rootPath string, path string) {

	c, found := getComponent(rm, repo, path)

	if found {
		fileMd5, err := getMd5(filepath.Join(rootPath, path))
		if err == nil && fileMd5 != c.Assets[0].Checksum.Md5 {
			fmt.Println("Uploading ", path)
			UploadFile(rm, repo, rootPath, path)
		} else {
			if err != nil {
				fmt.Println("Skipping ", c.Name, err)
			}
			if fileMd5 == c.Assets[0].Checksum.Md5 {
				fmt.Println("Skipping ", c.Name, ": Unchanged")
			}
		}
	} else {
		fmt.Println("Uploading ", path)
		UploadFile(rm, repo, rootPath, path)
	}

}

// UploadFile uploads a file to repo
func UploadFile(rm nexusrm.RM, repo string, rootPath string, path string) {
	f, err := os.Open(filepath.Join(rootPath, path))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	component := nexusrm.UploadComponentRaw{
		Assets: []nexusrm.UploadAssetRaw{
			nexusrm.UploadAssetRaw{
				File:     f,
				Filename: filepath.Base(path)}},
		Directory: filepath.Dir(path),
		Tag:       ""}
	err = nexusrm.UploadComponent(rm, repo, component)
	if err != nil {
		log.Fatal(err)
	}
}

// UploadAll files overwriting
func UploadAll(rm nexusrm.RM, repo string, rootPath string) {
	files, err := getLocalFiles(rootPath)
	if err != nil {
		log.Println(err)
	}
	for _, f := range files {
		UploadIfDifferent(rm, repo, rootPath, f)
	}
}

func main() {

	rootPath, err := promtInput("Root: ", "/home/aridane/NexusDrive")
	nexusServer, err := promtInput("Server: ", "http://localhost:8081")
	nexusUser, err := promtInput("User: ", "admin")
	nexusPassword, err := promtInput("Password: ", "admin")

	// Connect to nexus
	rm, err := nexusrm.New(nexusServer, nexusUser, nexusPassword)
	if err != nil {
		panic(err)
	}

	// Get list of repos
	var repoNames []string
	if repos, err := nexusrm.GetRepositories(rm); err == nil {
		for _, r := range repos {
			repoNames = append(repoNames, r.Name)
		}
	}

	// Select which repo to work with
	selectedRepo, err := promtSelect("Select Repo", repoNames, "")

	actions := []string{"List repo", "List local", "Download all", "Upload all"}
	action, err := promtSelect("Select Actions", actions, "")

	switch action {
	case actions[0]:
		ListRepo(rm, selectedRepo)
	case actions[1]:
		ListLocal(rootPath)
	case actions[2]:
		DownloadAll(rm, selectedRepo, rootPath)
	case actions[3]:
		UploadAll(rm, selectedRepo, rootPath)
	}

}
