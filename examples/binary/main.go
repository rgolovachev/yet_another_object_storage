package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func sendBinary() {
	filePath := "./hello"
	// you should change manually port here if you have changed config.json
	// bucket b must be created beforehand
	url := "http://0.0.0.0:18100/b/hello" // URL для загрузки бинарного файла

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	req, err := http.NewRequest("POST", url, file)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("status: %s\n", resp.Status)
}

func getBinary() {
	// change port here to actual API port
	bucket_url := "http://0.0.0.0:18100/b"
	http.Post(bucket_url, "", nil)
	binaryURL := "http://0.0.0.0:18100/b/hello"
	outputPath := "./downloaded_hello"

	response, err := http.Get(binaryURL)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
	// need to be executable
	file.Chmod(0755)
}

func main() {
	// choose one of them
	sendBinary()
	// getBinary()
}
