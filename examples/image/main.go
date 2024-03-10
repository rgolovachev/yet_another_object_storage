package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
)

func sendPic() {
	imagePath := "golang.png"

	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	buf := new(bytes.Buffer)
	err = png.Encode(buf, img)
	if err != nil {
		log.Fatal(err)
	}

	// change port here to actual API port
	bucket_url := "http://0.0.0.0:18100/b"
	http.Post(bucket_url, "", nil)
	url := "http://0.0.0.0:18100/b/golang.png"
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "image/png")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("status: %s\n", resp.Status)
}

func getPic() {
	// change port here to actual API port
	imageURL := "http://0.0.0.0:18100/b/golang.png"
	outputPath := "downloaded_golang_pic.png"

	response, err := http.Get(imageURL)
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
}

func main() {
	// choose one of two methods
	sendPic()
	// getPic()
}
