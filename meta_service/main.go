package main

import (
	"common"
	"database/sql"
	"log"
	"meta/meta"
	metapb "meta/proto"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/lib/pq"
)

const (
	dbConnStr         = "user=meta_service password=super_secret_pass dbname=meta_db host=meta_db port=5432 sslmode=disable"
	filesTableSchema  = "(id SERIAL PRIMARY KEY, bucket TEXT, file TEXT, content_type TEXT)"
	chunksTableSchema = "(id SERIAL PRIMARY KEY, file TEXT, chunk TEXT, shard TEXT)"
)

func main() {
	log.Println("meta service is started")
	meta_port := common.ReadConfig().Meta_port

	if meta_port == 0 {
		log.Fatalf("failed to start meta service: meta_port is not specified in config.json")
	}

	port := strconv.Itoa(meta_port)

	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	metaService := meta.NewServer()
	metapb.RegisterApiWithMetaServiceServer(grpcServer, metaService)

	metaService.DB, err = sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("troubles with connecting to db: %v\n", err)
	}
	defer metaService.DB.Close()

	_, err = metaService.DB.Exec("CREATE TABLE IF NOT EXISTS files " + filesTableSchema)
	if err != nil {
		log.Fatalf("troubles with creating files table: %s\n", err)
	}

	_, err = metaService.DB.Exec("CREATE TABLE IF NOT EXISTS chunks " + chunksTableSchema)
	if err != nil {
		log.Fatalf("troubles with creating chunks table: %s\n", err)
	}

	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("meta service failed")
	}
}
