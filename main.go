package main

import (
	"bytes"
	"context"
	"fmt"
	uploadpb "github/michaellimmm/upload-file-server/generated"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"cloud.google.com/go/storage"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	bucketName = "file-service-test"
	publicHost = "https://storage.googleapis.com"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Start CPU profiling
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		fmt.Printf("failed to create cpu.prof file, error: %+v\n", err)
		return
	}
	defer cpuFile.Close()
	pprof.StartCPUProfile(cpuFile)
	defer pprof.StopCPUProfile()

	// Start memory profiling
	memFile, err := os.Create("mem.prof")
	if err != nil {
		fmt.Printf("failed to create mem.prof file, error: %+v\n", err)
		return
	}
	defer memFile.Close()
	defer pprof.WriteHeapProfile(memFile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Printf("failed to initilize storage client, error: %+v", err)
		return
	}

	handler := NewFileServiceServer(storageClient)

	g := grpc.NewServer()
	reflection.Register(g)
	uploadpb.RegisterFileServiceServer(g, handler)
	port := ":8080"
	listen, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("failed to listen port %s", port)
		return
	}

	go func() {
		fmt.Println("GRPC server is running...")
		if err := g.Serve(listen); err != nil {
			panic(err)
		}
	}()

	<-ctx.Done()
	g.GracefulStop()

	fmt.Println("Good Bye")
}

type FileServiceServer struct {
	uploadpb.UnimplementedFileServiceServer
	storageClient *storage.Client
}

func NewFileServiceServer(storageClient *storage.Client) *FileServiceServer {
	return &FileServiceServer{
		storageClient: storageClient,
	}
}

func (f *FileServiceServer) Upload(stream uploadpb.FileService_UploadServer) error {
	filename := ""
	var fileSize int

	ctx := stream.Context()

	var storageWriter *storage.Writer
	defer func() {
		if storageWriter != nil {
			if err := storageWriter.Close(); err != nil {
				fmt.Printf("failed to close storageWriter, err: %+v\n", err)
			}
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			url := fmt.Sprintf("%s/%s/%s", publicHost, bucketName, filename)
			return stream.SendAndClose(&uploadpb.FileUploadResponse{Url: url})
		}
		if err != nil {
			fmt.Printf("failed to receive file, err: %+v\n", err)
			return err
		}

		if filename == "" {
			filename = req.GetFileName()
			storageWriter = f.storageClient.Bucket(bucketName).Object(filename).NewWriter(ctx)
		}

		chunk := req.GetChunk()

		if storageWriter != nil {
			if _, err := io.Copy(storageWriter, bytes.NewReader(chunk)); err != nil {
				return err
			}
		}

		fileSize += len(chunk)
	}
}
