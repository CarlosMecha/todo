package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func getVar(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		if def == "" {
			fmt.Fprintf(os.Stderr, "$%s is not defined", key)
			os.Exit(1)
		}
		return def
	}
	return value
}

func getRemoteVersion(client *http.Client, addr, token string) time.Time {

	req, err := http.NewRequest(http.MethodHead, addr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating head request: %s", err.Error())
		os.Exit(2)
	}

	req.Header.Add("X-Auth-Access-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to do head request: %s", err.Error())
		os.Exit(2)
	}

	header := resp.Header["Last-Modified"]
	if len(header) == 0 || header[0] == "" {
		return time.Time{}
	}

	version, err := time.Parse(time.RFC1123, header[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to parse version: %s", err.Error())
		os.Exit(2)
	}

	return version
}

func getLocalVersion(file string) time.Time {
	info, err := os.Stat(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to stat file: %s", err.Error())
		os.Exit(2)
	}

	return info.ModTime()
}

func execEditor(editor, file string) {
	cmd := exec.Command(editor, file)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to run editor: %s", err.Error())
		os.Exit(2)
	}
}

func upload(client *http.Client, addr, token, file string) {
	version := getLocalVersion(file)

	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to open file: %s", err.Error())
		os.Exit(2)
	}
	defer fd.Close()

	req, err := http.NewRequest(http.MethodPut, addr, fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create put request: %s", err.Error())
		os.Exit(2)
	}

	req.Header.Add("X-Auth-Access-Token", token)
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Last-Modified", version.Format(time.RFC1123))

	if _, err := client.Do(req); err != nil {
		fmt.Fprintf(os.Stderr, "unable to do put request: %s", err.Error())
		os.Exit(2)
	}
}

func main() {

	addr := getVar("TODO_ADDR", "")
	file := getVar("TODO_FILE", "")
	token := getVar("TODO_TOKEN", "")
	editor := getVar("TODO_EDITOR", "vim")

	client := &http.Client{}

	localVersion := getLocalVersion(file)
	remoteVersion := getRemoteVersion(client, addr, token)

	// TODO: Resolve conflict automatically
	if localVersion.After(remoteVersion) {
		fmt.Fprintf(os.Stderr, "The local file is newer than the remote one, fix conflic and try again")
		os.Exit(3)
	}

	execEditor(editor, file)

	upload(client, addr, token, file)

}
