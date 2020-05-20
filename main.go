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

// ListLocal List all files in local folder
func ListLocal(rootPath string) {
	err := filepath.Walk(rootPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()

				h := md5.New()
				if _, err := io.Copy(h, f); err != nil {
					log.Fatal(err)
				}
				relPath, _ := filepath.Rel(rootPath, path)
				fmt.Print(relPath, " ")
				fmt.Printf("%x\n", h.Sum(nil))
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
}

// DownloadAll Download all files from repo overwriting changed ones
func DownloadAll(rm nexusrm.RM, repo string, rootPath string) {

	components, err := nexusrm.GetComponents(rm, repo)
	if err == nil {
		for _, c := range components {
			fmt.Println(c.Name, c.Assets[0].Checksum.Md5)
		}
	}
	for _, c := range components {
		fmt.Println("Downloading ", rootPath+"/"+c.Name)
		DownloadFile(rootPath+"/"+c.Name, c.Assets[0].DownloadURL)
	}

}

// UploadAll files overwriting
func UploadAll() {
	fmt.Println("Not developed yet :)")
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
		UploadAll()
	}

}
