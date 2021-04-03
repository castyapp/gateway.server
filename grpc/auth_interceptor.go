package grpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

func clientInterceptor(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	// Logic before invoking the invoker
	start := time.Now()
	// Calls the invoker to execute RPC
	err := invoker(ctx, method, req, reply, cc, opts...)
	// Logic after invoking the invoker
	log.Printf("Invoked RPC method=%s; Duration=%s; Error=%v", method, time.Since(start), err)
	return err
}

func WithAuthInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(clientInterceptor)
}
