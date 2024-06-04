package server

import (
	"bytes"
	"errors"
	"fmt"
	uploadpb "github/michaellimmm/upload-file-server/generated"
	"io"
	"net/http"

	"cloud.google.com/go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	bucketName = "file-service-test"
	publicHost = "https://storage.googleapis.com"
)

type FileServiceServer struct {
	uploadpb.UnimplementedFileServiceServer
	storageClient *storage.Client
	httpClient    *http.Client
}

func NewFileServiceServer(storageClient *storage.Client, httpClient *http.Client) *FileServiceServer {
	return &FileServiceServer{
		storageClient: storageClient,
		httpClient:    httpClient,
	}
}

func (f *FileServiceServer) Upload(stream uploadpb.FileService_UploadServer) (errResponse error) {
	filename := ""

	ctx := stream.Context()

	var storageWriter *storage.Writer
	defer func() {
		if storageWriter != nil {
			if err := storageWriter.Close(); err != nil {
				fmt.Printf("failed to close storageWriter, err: %+v\n", err)
				errResponse = status.Error(codes.Internal, err.Error())
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
			return status.Error(codes.Internal, err.Error())
		}

		if filename == "" {
			filename = req.GetFileName()
			if storageWriter == nil {
				storageWriter = f.storageClient.Bucket(bucketName).Object(filename).NewWriter(ctx)
			}
		}

		switch req.UploadType {
		case uploadpb.UploadSourceType_UPLOAD_SOURCE_TYPE_FILE:
			{
				chunk := req.GetChunk()
				if storageWriter != nil {
					if _, err := io.Copy(storageWriter, bytes.NewReader(chunk)); err != nil {
						fmt.Printf("failed to save image, err: %+v\n", err)
						return status.Error(codes.Internal, err.Error())
					}
				}
			}
		case uploadpb.UploadSourceType_UPLOAD_SOURCE_TYPE_URL:
			{
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, req.GetUrl(), nil)
				if err != nil {
					fmt.Printf("failed to create request, err: %+v\n", err)
					return status.Error(codes.Internal, err.Error())
				}

				res, err := f.httpClient.Do(req)
				if err != nil {
					fmt.Printf("failed to send request, err: %+v\n", err)
					return status.Error(codes.Internal, err.Error())
				}

				if res.StatusCode/100 != 2 {
					err = errors.New("failed to download file")
					fmt.Printf("response body is not success, status: %d\n", res.StatusCode)
					return status.Error(codes.Internal, err.Error())
				}

				if storageWriter != nil {
					if _, err := io.Copy(storageWriter, res.Body); err != nil {
						fmt.Printf("failed to save image, err: %+v\n", err)
						return status.Error(codes.Internal, err.Error())
					}
				}
			}
		default:
			return status.Error(codes.InvalidArgument, fmt.Sprintf("%s is invalid", req.UploadType.String()))
		}
	}
}
