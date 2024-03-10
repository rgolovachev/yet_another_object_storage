package common

import (
	"encoding/json"
	"os"
	"strconv"
)

const (
	Delimeter = ":"
)

type Config struct {
	Chunk_size int            `json:"chunk_size"`
	Api_port   int            `json:"api_port"`
	Meta_port  int            `json:"meta_port"`
	Stat_port  int            `json:"stat_port"`
	Shards     map[string]int `json:"storage_port"`
}

func ReadConfig() Config {
	var config Config
	config_path := "../config.json"
	fd, err := os.Open(config_path)
	if err != nil {
		panic("can't open config file")
	}
	defer fd.Close()

	file_info, err := fd.Stat()
	if err != nil {
		panic("can't get stat from config file")
	}

	config_raw := make([]byte, file_info.Size())
	_, err = fd.Read(config_raw)
	if err != nil {
		panic("can't read config file")
	}

	err = json.Unmarshal(config_raw, &config)
	if err != nil {
		panic("can't read config file")
	}
	return config
}

func GetChunkName(bucket, file string, seqnum int) string {
	return bucket + "_" + strconv.Itoa(seqnum) + "_" + file
}
