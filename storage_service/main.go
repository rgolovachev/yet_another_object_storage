package main

import (
	"common"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type shardServer struct {
	config    common.Config
	data_path string
	name      string
}

func (s *shardServer) writeData(w http.ResponseWriter, req *http.Request) {
	filename := mux.Vars(req)["filename"]
	path := s.data_path + filename

	body := make([]byte, req.ContentLength)
	_, err := req.Body.Read(body)
	if err != nil && err != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Troubles with reading data from request")
		return
	}

	fd, err := os.Create(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Can't create file")
		return
	}

	defer fd.Close()
	_, err = fd.Write(body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Can't write data into file")
		return
	}
}

func (s *shardServer) readData(w http.ResponseWriter, req *http.Request) {
	filename := mux.Vars(req)["filename"]
	path := s.data_path + filename

	fd, err := os.Open(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Can't open file")
		return
	}
	defer fd.Close()

	_, err = io.Copy(w, fd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/octet-stream")
}

func (s *shardServer) deleteData(w http.ResponseWriter, req *http.Request) {
	filename := mux.Vars(req)["filename"]
	path := s.data_path + filename

	err := os.Remove(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Can't remove file")
		return
	}
}

func (s *shardServer) getStats(w http.ResponseWriter, req *http.Request) {
	chunk_files, err := os.ReadDir(s.data_path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Received unexpected error while reading dir: %v\n", err)
		return
	}

	fmt.Fprintf(w, "There are %d chunks in %s shard:\n", len(chunk_files), s.name)
	longest_name_len := 0
	for _, chunk_file := range chunk_files {
		longest_name_len = max(longest_name_len, len(chunk_file.Name()))
	}

	fmt.Fprintf(w, "  Filename: %s Size:\n", strings.Repeat(" ", longest_name_len))
	for _, chunk_file := range chunk_files {
		file_info, err := chunk_file.Info()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while reading some file in dir: %v\n", err)
			return
		}

		fmt.Fprintf(w, "> %s%s%d bytes\n", chunk_file.Name(), strings.Repeat(" ", longest_name_len+11-len(chunk_file.Name())), file_info.Size())
	}
}

// pass shard name in command line argument and create new foler data_<shard_name>
func main() {
	if len(os.Args) != 2 {
		log.Fatalln("fatal error: You must specify shard name")
	}

	shard_server := &shardServer{}
	shard_server.name = os.Args[1]
	log.Printf("storage service is started (shard %s)\n", shard_server.name)
	shard_server.config = common.ReadConfig()
	port, ok := shard_server.config.Shards[shard_server.name]
	if !ok {
		log.Fatalf("fatal error: unknown shard name: %s\n", shard_server.name)
	}

	// it's ok if there is existing data directory
	shard_server.data_path = "./data_" + shard_server.name + "/"
	os.Mkdir(shard_server.data_path, 0755)

	r := mux.NewRouter()

	r.HandleFunc("/{filename}", shard_server.writeData).Methods("POST")
	r.HandleFunc("/{filename}", shard_server.readData).Methods("GET")
	r.HandleFunc("/{filename}", shard_server.deleteData).Methods("DELETE")
	r.HandleFunc("/stats/get", shard_server.getStats).Methods("GET")

	http.ListenAndServe(":"+strconv.Itoa(port), r)
}
