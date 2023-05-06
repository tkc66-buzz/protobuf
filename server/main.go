package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"protobuf/pb"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedFileServiceServer
}

func (s *server) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	fmt.Println("ListFiles was invoked")
	dir := "/Users/takeshiwatanabe/EureWorks/udemy/protobuf/storage"
	paths, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var filenames []string
	for _, path := range paths {
		if !path.IsDir() {
			filenames = append(filenames, path.Name())
		}
	}
	res := &pb.ListFilesResponse{
		Filenames: filenames,
	}
	return res, nil
}

func (s *server) Download(req *pb.DownloadRequest, stream pb.FileService_DownloadServer) error {
	fmt.Println("Download was invoked")
	filename := req.GetFilename()
	path := "/Users/takeshiwatanabe/EureWorks/udemy/protobuf/storage/" + filename

	if _, err := os.Stat(path); err != nil {
		return status.Errorf(codes.NotFound, "file %s doesn't exist", filename)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	buf := make([]byte, 5)
	for {
		n, err := file.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 || err == io.EOF {
			break
		}
		res := &pb.DownloadResponse{Data: buf[:n]}
		sendErr := stream.Send(res)
		if sendErr != nil {
			return sendErr
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func (*server) Upload(stream pb.FileService_UploadServer) error {
	fmt.Println("Upload was invoked")
	var buf bytes.Buffer
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			res := &pb.UploadResponse{Size: int32(buf.Len())}
			return stream.SendAndClose(res)
		}
		if err != nil {
			return err
		}
		log.Println(res.GetData())
		buf.Write(res.GetData())
	}
}

func (*server) UploadAndNotifuProgress(stream pb.FileService_UploadAndNotifyProgressServer) error {
	fmt.Println("UploadAndNotifuProgress was invoked")
	size := 0

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		data := req.GetData()
		log.Println(data)
		size += len(data)

		res := &pb.UploadAndNotifyProgressResponse{
			Message: fmt.Sprintf("recieved data size: %v bytes", size),
		}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
}

func logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		log.Printf("gRPC method: %s, %v", info.FullMethod, req)
		return handler(ctx, req)
	}
}

func authorize(ctx context.Context) (context.Context, error) {
	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}
	if token != "test_token" {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return ctx, nil
}

func main() {
	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v¥n", err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(
		grpc_middleware.ChainUnaryServer(
			logging(),
			grpc_auth.UnaryServerInterceptor(authorize))))
	pb.RegisterFileServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v¥n", err)
	}

}
