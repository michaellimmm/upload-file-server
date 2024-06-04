package main

import (
	"context"
	"fmt"
	uploadpb "github/michaellimmm/upload-file-server/generated"
	"github/michaellimmm/upload-file-server/server"
	"log"
	"net"
	"net/http"
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

	handler := server.NewFileServiceServer(storageClient, http.DefaultClient)

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
