package meta

import (
	"context"
	"database/sql"
	metapb "meta/proto"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	metapb.UnimplementedApiWithMetaServiceServer
	DB *sql.DB
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) CreateBucket(ctx context.Context, req *metapb.CreateBucketReq) (*metapb.CreateBucketResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.CreateBucketResp{}, status.Errorf(codes.Internal, "failed to begin tx while creating bucket %s", req.Bucket)
	}
	defer tx.Rollback()

	count := 0
	tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE bucket = $1", req.Bucket).Scan(&count)
	if count > 0 {
		return &metapb.CreateBucketResp{}, status.Errorf(codes.AlreadyExists, "bucket with name %s already exists", req.Bucket)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO files (bucket, file, content_type) VALUES ($1, $2, $3)", req.Bucket, sql.NullString{}, sql.NullString{})
	if err != nil {
		return &metapb.CreateBucketResp{}, status.Errorf(codes.Internal, "failed to insert row into files table while creating bucket %s", req.Bucket)
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.CreateBucketResp{}, status.Errorf(codes.Internal, "failed to commit tx while creating bucket %s", req.Bucket)
	}

	return &metapb.CreateBucketResp{}, nil
}

func (s *Server) DeleteBucket(ctx context.Context, req *metapb.DeleteBucketReq) (*metapb.DeleteBucketResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.DeleteBucketResp{}, status.Errorf(codes.Internal, "failed to begin tx while deleting bucket %s", req.Bucket)
	}
	defer tx.Rollback()

	count := 0
	tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE bucket = $1", req.Bucket).Scan(&count)

	if count == 0 {
		return &metapb.DeleteBucketResp{}, status.Errorf(codes.NotFound, "bucket with name %s does not exist", req.Bucket)
	}
	if count > 1 {
		return &metapb.DeleteBucketResp{}, status.Errorf(codes.FailedPrecondition, "bucket %s is not empty before deleting", req.Bucket)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM files WHERE bucket = $1", req.Bucket)
	if err != nil {
		return &metapb.DeleteBucketResp{}, status.Errorf(codes.Internal, "failed while processing DELETE query while deleting bucket %s", req.Bucket)
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.DeleteBucketResp{}, status.Errorf(codes.Internal, "failed to commit tx while deleting bucket %s", req.Bucket)
	}

	return &metapb.DeleteBucketResp{}, nil
}

func (s *Server) GetFiles(ctx context.Context, req *metapb.GetFilesReq) (*metapb.GetFilesResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.GetFilesResp{}, status.Errorf(codes.Internal, "failed to begin tx while getting list of files from bucket %s", req.Bucket)
	}
	defer tx.Rollback()

	rows, err := s.DB.QueryContext(ctx, "SELECT file FROM files WHERE bucket = $1", req.Bucket)
	if err != nil {
		return &metapb.GetFilesResp{}, status.Errorf(codes.Internal, "failed while processing SELECT query while getting list of files of bucket %s", req.Bucket)
	}
	defer rows.Close()

	var cur_file string
	list_of_files := make([]string, 0)
	bucket_exists := false

	for rows.Next() {
		err = rows.Scan(&cur_file)
		// it's a terrible crutch but i haven't figured out how to make it better
		if err != nil && !strings.Contains(err.Error(), "converting NULL to string is unsupported") {
			return &metapb.GetFilesResp{}, status.Errorf(codes.Internal, "failed while reading results of SELECT query while getting list of files of bucket %s", req.Bucket)
		}
		bucket_exists = true
		// "" <- the "file" is NULL
		if strings.Compare(cur_file, "") != 0 {
			list_of_files = append(list_of_files, cur_file)
		}
	}

	if !bucket_exists {
		return &metapb.GetFilesResp{}, status.Errorf(codes.NotFound, "bucket with name %s does not exist", req.Bucket)
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.GetFilesResp{}, status.Errorf(codes.Internal, "failed to commit tx while getting files from bucket %s", req.Bucket)
	}

	return &metapb.GetFilesResp{Files: list_of_files}, nil
}

func (s *Server) CreateFile(ctx context.Context, req *metapb.CreateFileReq) (*metapb.CreateFileResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed to begin tx while creating file %s in bucket %s", req.File, req.Bucket)
	}
	defer tx.Rollback()

	rows, err := s.DB.QueryContext(ctx, "SELECT file FROM files WHERE bucket = $1", req.Bucket)
	if err != nil {
		return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed while processing SELECT query while creating file %s in bucket %s", req.File, req.Bucket)
	}
	defer rows.Close()

	var cur_file string
	bucket_exists := false

	for rows.Next() {
		err = rows.Scan(&cur_file)
		if err != nil && !strings.Contains(err.Error(), "converting NULL to string is unsupported") {
			return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed while reading results of SELECT query while creating file %s in bucket %s: %v", req.File, req.Bucket, err)
		}
		bucket_exists = true
		if strings.Compare(cur_file, req.File) == 0 {
			return &metapb.CreateFileResp{}, status.Errorf(codes.AlreadyExists, "file with name %s already exists in bucket %s", req.File, req.Bucket)
		}
	}

	if !bucket_exists {
		return &metapb.CreateFileResp{}, status.Errorf(codes.NotFound, "bucket with name %s does not exist", req.Bucket)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO files (bucket, file, content_type) VALUES ($1, $2, $3)", req.Bucket, req.File, req.ContentType)
	if err != nil {
		return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed to insert row into files table while creating file %s in bucket %s", req.File, req.Bucket)
	}

	for i := 0; i < len(req.Chunks); i++ {
		cur_chunk := req.Chunks[i].Filename
		cur_shard := req.Chunks[i].Shard
		_, err = tx.ExecContext(ctx, "INSERT INTO chunks (file, chunk, shard) VALUES ($1, $2, $3)", req.File, cur_chunk, cur_shard)
		if err != nil {
			return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed to insert row into chunks table while creating file %s in bucket %s", req.File, req.Bucket)
		}
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.CreateFileResp{}, status.Errorf(codes.Internal, "failed to commit tx while creating file %s in bucket %s", req.File, req.Bucket)
	}

	return &metapb.CreateFileResp{}, nil
}

func (s *Server) DeleteFile(ctx context.Context, req *metapb.DeleteFileReq) (*metapb.DeleteFileResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed to begin tx while deleting file %s in bucket %s", req.File, req.Bucket)
	}
	defer tx.Rollback()

	count := 0
	err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE bucket = $1 AND file = $2", req.Bucket, req.File).Scan(&count)
	if count == 0 {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.NotFound, "bucket with name %s does not exist or file with name %s does not exist", req.Bucket, req.File)
	} else if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "unknown error while deleting file %s from bucket %s: %v", req.File, req.Bucket, err)
	}

	rows_from_chunks, err := s.DB.QueryContext(ctx, "SELECT chunk, shard FROM chunks WHERE file = $1", req.File)
	if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed while processing SELECT query while deleting file %s in bucket %s from chunks table", req.File, req.Bucket)
	}
	defer rows_from_chunks.Close()

	var cur_chunk, cur_shard string
	chunks_with_shards := make([]*metapb.ChunkFilenameWithShard, 0)

	for rows_from_chunks.Next() {
		err = rows_from_chunks.Scan(&cur_chunk, &cur_shard)
		if err != nil {
			return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed while reading results of SELECT query while deleting file %s in bucket %s: %v", req.File, req.Bucket, err)
		}
		chunks_with_shards = append(chunks_with_shards, &metapb.ChunkFilenameWithShard{Filename: cur_chunk, Shard: cur_shard})
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM files WHERE file = $1", req.File)
	if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed while processing DELETE query while deleting file %s from files table", req.File)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM chunks WHERE file = $1", req.File)
	if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed while processing DELETE query while deleting file %s from chunks table", req.File)
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.DeleteFileResp{}, status.Errorf(codes.Internal, "failed to commit tx while deleting file %s in bucket %s", req.File, req.Bucket)
	}

	return &metapb.DeleteFileResp{Chunks: chunks_with_shards}, nil
}

func (s *Server) GetFileChunks(ctx context.Context, req *metapb.GetFileChunksReq) (*metapb.GetFileChunksResp, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return &metapb.GetFileChunksResp{}, status.Errorf(codes.Internal, "failed to begin tx while getting file chunks %s in bucket %s", req.File, req.Bucket)
	}
	defer tx.Rollback()

	content_type := ""
	err = s.DB.QueryRowContext(ctx, "SELECT content_type FROM files WHERE bucket = $1 AND file = $2", req.Bucket, req.File).Scan(&content_type)
	if err != nil && strings.Compare(err.Error(), "sql: no rows in result set") == 0 {
		return &metapb.GetFileChunksResp{}, status.Errorf(codes.NotFound, "bucket with name %s does not exist or file with name %s does not exist", req.Bucket, req.File)
	} else if err != nil {
		return &metapb.GetFileChunksResp{}, status.Errorf(codes.Internal, "unknown error while getting chunks from file %s from bucket %s: %v", req.File, req.Bucket, err)
	}

	rows_from_chunks, err := s.DB.QueryContext(ctx, "SELECT chunk, shard FROM chunks WHERE file = $1", req.File)
	if err != nil {
		return &metapb.GetFileChunksResp{}, status.Errorf(codes.Internal, "failed while processing SELECT query while getting chunks of file %s in bucket %s from chunks table", req.File, req.Bucket)
	}
	defer rows_from_chunks.Close()

	var cur_chunk, cur_shard string
	chunks_with_shards := make([]*metapb.ChunkFilenameWithShard, 0)

	for rows_from_chunks.Next() {
		err = rows_from_chunks.Scan(&cur_chunk, &cur_shard)
		if err != nil {
			return &metapb.GetFileChunksResp{}, status.Errorf(codes.Internal, "failed while reading results of SELECT query while getting chunks of file %s in bucket %s: %v", req.File, req.Bucket, err)
		}
		chunks_with_shards = append(chunks_with_shards, &metapb.ChunkFilenameWithShard{Filename: cur_chunk, Shard: cur_shard})
	}

	err = tx.Commit()
	if err != nil {
		return &metapb.GetFileChunksResp{}, status.Errorf(codes.Internal, "failed to commit tx while getting chunks of file %s in bucket %s", req.File, req.Bucket)
	}

	return &metapb.GetFileChunksResp{Chunks: chunks_with_shards, ContentType: content_type}, nil
}
