package main

import (
	"bytes"
	"common"
	"context"
	"fmt"
	"io"
	"log"
	metapb "meta/proto"
	"net/http"
	"strconv"
	"strings"

	"bitbucket.org/pcastools/hash"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type apiServer struct {
	conn        *grpc.ClientConn
	grpc_client metapb.ApiWithMetaServiceClient
	config      common.Config
}

// rendezvous hashing
func (s *apiServer) getShard(chunk []byte) (string, int) {
	var best uint32
	var best_shard_name string
	var best_shard_port int
	for shard, port := range s.config.Shards {
		cur := hash.ByteSlice(chunk) ^ hash.String(shard)
		if best < cur {
			best = cur
			best_shard_name = shard
			best_shard_port = port
		}
	}
	return best_shard_name, best_shard_port
}

func (s *apiServer) createBucket(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]

	if strings.Contains(bucket, common.Delimeter) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Name of bucket must not contain delimeter symbol %s\n", common.Delimeter)
		return
	}
	if strings.Compare(bucket, "") == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Name of bucket must be non-empty")
		return
	}

	_, err := s.grpc_client.CreateBucket(context.Background(), &metapb.CreateBucketReq{Bucket: bucket})

	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			w.WriteHeader(http.StatusPreconditionFailed)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			log.Fatalf("Received unknown error in createBucket: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	fmt.Fprintf(w, "Successfuly created bucket: %s\n", bucket)
	log.Printf("Created bucket: %s\n", bucket)
}

func (s *apiServer) deleteBucket(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]

	_, err := s.grpc_client.DeleteBucket(context.Background(), &metapb.DeleteBucketReq{Bucket: bucket})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
		case codes.FailedPrecondition:
			w.WriteHeader(http.StatusPreconditionFailed)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			log.Fatalf("Received unknown error in deleteBucket: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	fmt.Fprintf(w, "Successfuly deleted bucket: %s\n", bucket)
	log.Printf("Deleted bucket: %s\n", bucket)
}

func (s *apiServer) getFilesFromBucket(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]

	resp, err := s.grpc_client.GetFiles(context.Background(), &metapb.GetFilesReq{Bucket: bucket})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			log.Fatalf("Received unknown error in getFilesFromBucket: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	log.Println("Served getFilesFromBucket response")
	fmt.Fprintf(w, "Bucket %s consists from files:\n", bucket)
	for i := 0; i < len(resp.Files); i++ {
		fmt.Fprintf(w, "> %s\n", resp.Files[i])
	}
}

func (s *apiServer) createFile(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]
	file := mux.Vars(req)["file"]
	content_type := req.Header.Get("Content-Type")

	if strings.Contains(file, common.Delimeter) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Name of file must not contain delimeter symbol %s\n", common.Delimeter)
		return
	}
	if strings.Compare(file, "") == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Name of file must be non-empty")
		return
	}

	chunk := make([]byte, s.config.Chunk_size)
	seqnum := 0
	req_to_meta := &metapb.CreateFileReq{Bucket: bucket, File: file, ContentType: content_type, Chunks: make([]*metapb.ChunkFilenameWithShard, 0)}

	n, err := req.Body.Read(chunk)
	for ; !(err != nil && err != io.EOF); n, err = req.Body.Read(chunk) {
		shard_name, shard_port := s.getShard(chunk)
		chunk_name := common.GetChunkName(bucket, file, seqnum)
		req_to_meta.Chunks = append(req_to_meta.Chunks, &metapb.ChunkFilenameWithShard{Filename: chunk_name, Shard: shard_name})

		_, http_err := http.Post(s.getStorageHandler(shard_name, shard_port, chunk_name), "application/octet-stream", bytes.NewBuffer(chunk[:n]))
		if http_err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while sending data to shards: %v\n", http_err)
			return
		}
		seqnum++
		if err == io.EOF {
			break
		}
	}

	if err != nil && err != io.EOF {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Received unexpected error while reading data from request: %s\n", err)
		return
	}

	_, err = s.grpc_client.CreateFile(context.Background(), req_to_meta)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		case codes.AlreadyExists:
			w.WriteHeader(http.StatusPreconditionFailed)
			fmt.Fprintln(w, "Updating files is not supported")
			return
		default:
			log.Fatalf("Received unknown error createFile: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	fmt.Fprintf(w, "Successfully created file %s in bucket %s\n", file, bucket)
	log.Printf("Create file %s in bucket %s\n", file, bucket)
}

func (s *apiServer) deleteFile(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]
	file := mux.Vars(req)["file"]

	chunks, err := s.grpc_client.DeleteFile(context.Background(), &metapb.DeleteFileReq{Bucket: bucket, File: file})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			log.Fatalf("Received unknown error in deleteFile: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	for i := 0; i < len(chunks.Chunks); i++ {
		chunk_name := chunks.Chunks[i].Filename
		shard_name := chunks.Chunks[i].Shard
		shard_port := s.config.Shards[shard_name]
		delete_req, err := http.NewRequest("DELETE", s.getStorageHandler(shard_name, shard_port, chunk_name), nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while writing data to shards: %s\n", err)
			return
		}
		client := &http.Client{}
		_, err = client.Do(delete_req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while writing data to shards: %s\n", err)
			return
		}

	}

	fmt.Fprintf(w, "Successfully deleted file %s in bucket %s\n", file, bucket)
	log.Printf("Delete file %s in bucket %s\n", file, bucket)
}

func (s *apiServer) getFile(w http.ResponseWriter, req *http.Request) {
	bucket := mux.Vars(req)["bucket"]
	file := mux.Vars(req)["file"]

	resp, err := s.grpc_client.GetFileChunks(context.Background(), &metapb.GetFileChunksReq{Bucket: bucket, File: file})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
		case codes.Internal:
			w.WriteHeader(http.StatusInternalServerError)
		case codes.Unavailable:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			log.Fatalf("Received unknown error in getFile: %v\n", err)
		}
		fmt.Fprintf(w, "Received error: %v\n", err)
		return
	}

	for i := 0; i < len(resp.Chunks); i++ {
		chunk_name := resp.Chunks[i].Filename
		shard_name := resp.Chunks[i].Shard
		shard_port := s.config.Shards[shard_name]
		data_req, err := http.Get(s.getStorageHandler(shard_name, shard_port, chunk_name))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while reading data from chunks: %v\n", err)
			return
		}
		body, err := io.ReadAll(data_req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Received unexpected error while reading data from chunks: %v\n", err)
			return
		}
		w.Write(body)
	}

	w.Header().Add("Content-Type", resp.ContentType)
	log.Printf("Read file %s in bucket %s\n", file, bucket)
}

func (s *apiServer) getMetaAddr() string {
	return "dns:///meta_service:" + strconv.Itoa(s.config.Meta_port)
}

func (s *apiServer) getAPIAddr() string {
	if s.config.Api_port == 0 {
		log.Fatalln("You must specify port for API service")
	}

	return ":" + strconv.Itoa(s.config.Api_port)
}

func (s *apiServer) getStorageHandler(shard_name string, shard_port int, chunk_name string) string {
	return "http://" + shard_name + ":" + strconv.Itoa(shard_port) + "/" + chunk_name
}

func main() {
	log.Println("api service is started")
	r := mux.NewRouter()

	var api_server apiServer
	var err error
	api_server.config = common.ReadConfig()
	api_server.conn, err = grpc.Dial(api_server.getMetaAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer api_server.conn.Close()

	api_server.grpc_client = metapb.NewApiWithMetaServiceClient(api_server.conn)

	r.HandleFunc("/{bucket}", api_server.createBucket).Methods("POST")
	r.HandleFunc("/{bucket}", api_server.deleteBucket).Methods("DELETE")
	r.HandleFunc("/{bucket}", api_server.getFilesFromBucket).Methods("GET")
	r.HandleFunc("/{bucket}/{file}", api_server.createFile).Methods("POST")
	r.HandleFunc("/{bucket}/{file}", api_server.deleteFile).Methods("DELETE")
	r.HandleFunc("/{bucket}/{file}", api_server.getFile).Methods("GET")

	http.ListenAndServe(api_server.getAPIAddr(), r)
}
