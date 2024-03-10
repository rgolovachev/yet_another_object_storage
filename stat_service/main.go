package main

import (
	"common"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type statServer struct {
	config common.Config
}

func getShardURL(shard string, port int) string {
	return "http://" + shard + ":" + strconv.Itoa(port) + "/stats/get"
}

func (s *statServer) getStatsFromShard(w http.ResponseWriter, req *http.Request) {
	shard := mux.Vars(req)["shard"]

	port, exists := s.config.Shards[shard]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unknown shard %s\n", shard)
		return
	}

	resp, err := http.Get(getShardURL(shard, port))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unexpected error while getting stats from shard %s: %v\n", shard, err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unexpected error while getting stats from shard %s: %v\n", shard, err)
		return
	}

	w.Write(body)
}

func main() {
	log.Println("stat server is started")
	stat_server := &statServer{config: common.ReadConfig()}

	if stat_server.config.Stat_port == 0 {
		log.Fatalln("You must specify port for statistics service")
	}

	r := mux.NewRouter()

	r.HandleFunc("/stat/shard/{shard}", stat_server.getStatsFromShard).Methods("GET")

	http.ListenAndServe(":"+strconv.Itoa(stat_server.config.Stat_port), r)
}
